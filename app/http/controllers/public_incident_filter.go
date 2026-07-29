package controllers

import (
	"encoding/json"
	"fmt"
	"strings"

	"goravel/app/models"
)

type publicIncidentFilter struct {
	ScopePageID  string
	Limit        int
	ShowResolved bool
	SourceTypes  map[string]struct{}
	MonitorIDs   map[string]struct{}
	ServerIDs    map[string]struct{}
	Empty        bool // 页面无事件块，返回空列表
}

func isIncidentsOnlyPage(page PublicPageV1) bool {
	if len(page.Blocks) == 0 {
		return false
	}
	for _, b := range page.Blocks {
		if b.Type != "incidents" {
			return false
		}
	}
	return true
}

func companionIncidentsPath(statusPath string) string {
	base := strings.TrimRight(strings.TrimSpace(statusPath), "/")
	if base == "" || base == "/public" {
		return "/public/incidents"
	}
	return base + "/incidents"
}

func resolveIncidentScopePageID(page PublicPageV1) string {
	return page.ID
}

func inheritMonitorIDsFromPage(page PublicPageV1) []uint {
	for _, b := range page.Blocks {
		if b.Type != "serviceStatus" {
			continue
		}
		var d PublicBlockServiceStatusV1
		if err := json.Unmarshal(b.Data, &d); err != nil {
			return nil
		}
		out := make([]uint, 0, len(d.MonitorIDs))
		for _, id := range d.MonitorIDs {
			if id > 0 {
				out = append(out, id)
			}
		}
		return out
	}
	return nil
}

func pageHasStatusBlocks(page PublicPageV1) bool {
	for _, b := range page.Blocks {
		if b.Type == "serviceStatus" || b.Type == "serverList" {
			return true
		}
	}
	return false
}

func findPageIncidentBlock(page PublicPageV1) (PublicBlockIncidentsV1, bool) {
	for _, b := range page.Blocks {
		if b.Type != "incidents" {
			continue
		}
		d := PublicBlockIncidentsV1{ShowResolved: true}
		if err := json.Unmarshal(b.Data, &d); err != nil {
			return PublicBlockIncidentsV1{}, false
		}
		return d, true
	}
	return PublicBlockIncidentsV1{}, false
}

func defaultIncidentBlockData() PublicBlockIncidentsV1 {
	return PublicBlockIncidentsV1{
		Limit:        20,
		ShowResolved: true,
		SourceTypes:  []string{},
		MonitorIDs:   []uint{},
		ServerIDs:    []string{},
	}
}

/** 将历史独立事件页合并回状态页，保证一页绑定状态+事件 */
func ensurePublicIncidentsSeparated(cfg *PublicPagesConfigV1) {
	if cfg == nil {
		return
	}
	byPath := make(map[string]PublicPageV1, len(cfg.Pages))
	for _, page := range cfg.Pages {
		byPath[page.Path] = page
	}

	mergedCompanionIDs := map[string]struct{}{}
	pages := make([]PublicPageV1, 0, len(cfg.Pages))

	for _, page := range cfg.Pages {
		if isIncidentsOnlyPage(page) {
			continue
		}

		blocks := append([]PublicPageBlockV1{}, page.Blocks...)
		_, hasIncident := findPageIncidentBlock(page)
		incidentsPath := companionIncidentsPath(page.Path)
		if companion, ok := byPath[incidentsPath]; ok && isIncidentsOnlyPage(companion) && !hasIncident {
			incidentData := PublicBlockIncidentsV1{ShowResolved: true}
			if len(companion.Blocks) > 0 {
				_ = json.Unmarshal(companion.Blocks[0].Data, &incidentData)
			}
			inherited := inheritMonitorIDsFromPage(page)
			if len(incidentData.MonitorIDs) == 0 && len(inherited) > 0 {
				incidentData.MonitorIDs = inherited
			}
			blocks = append(blocks, mustBlock("incidents", incidentData))
			hasIncident = true
			mergedCompanionIDs[companion.ID] = struct{}{}
		}

		if pageHasStatusBlocks(PublicPageV1{Blocks: blocks}) && !hasIncident {
			incidentData := defaultIncidentBlockData()
			inherited := inheritMonitorIDsFromPage(page)
			if len(inherited) > 0 {
				incidentData.MonitorIDs = inherited
			}
			blocks = append(blocks, mustBlock("incidents", incidentData))
		}

		page.Blocks = blocks
		pages = append(pages, page)
	}

	for _, page := range cfg.Pages {
		if !isIncidentsOnlyPage(page) {
			continue
		}
		if _, ok := mergedCompanionIDs[page.ID]; ok {
			continue
		}
		parentPath := ""
		path := strings.TrimRight(page.Path, "/")
		if path == "/public/incidents" {
			parentPath = "/public"
		} else if strings.HasSuffix(path, "/incidents") {
			parentPath = strings.TrimSuffix(path, "/incidents")
			if parentPath == "" {
				parentPath = "/public"
			}
		}
		hasParent := false
		for _, p := range pages {
			if p.Path == parentPath {
				hasParent = true
				break
			}
		}
		if hasParent {
			continue
		}
		pages = append(pages, page)
	}

	cfg.Pages = pages
}

func resolveBoundPageByPath(cfg PublicPagesConfigV1, path string) (*PublicPageV1, error) {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	if path == "" {
		return nil, fmt.Errorf("path 不能为空")
	}

	for i := range cfg.Pages {
		if cfg.Pages[i].Path == path || strings.TrimRight(cfg.Pages[i].Path, "/") == path {
			return &cfg.Pages[i], nil
		}
	}

	// /public/xxx/incidents → 绑定到父状态页
	if strings.HasSuffix(path, "/incidents") {
		parentPath := "/public"
		if path != "/public/incidents" {
			parentPath = strings.TrimSuffix(path, "/incidents")
			if parentPath == "" {
				parentPath = "/public"
			}
		}
		for i := range cfg.Pages {
			p := strings.TrimRight(cfg.Pages[i].Path, "/")
			if p == parentPath {
				return &cfg.Pages[i], nil
			}
		}
	}

	return nil, fmt.Errorf("公开页面不存在")
}

func buildPublicIncidentFilterForPath(path string) (*publicIncidentFilter, error) {
	resolution, err := getPublicPagePolicy().Resolve(path)
	if err != nil {
		return nil, err
	}
	return resolution.IncidentFilter, nil
}

func buildPublicIncidentFilter(page *PublicPageV1) *publicIncidentFilter {
	if page == nil {
		return &publicIncidentFilter{Empty: true}
	}
	scopePageID := resolveIncidentScopePageID(*page)
	block, ok := findPageIncidentBlock(*page)
	if !ok {
		return &publicIncidentFilter{ScopePageID: scopePageID, Empty: true}
	}

	limit := block.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	sourceTypes := map[string]struct{}{}
	for _, t := range block.SourceTypes {
		t = strings.TrimSpace(t)
		if t != "" {
			sourceTypes[t] = struct{}{}
		}
	}
	monitorIDs := map[string]struct{}{}
	for _, id := range block.MonitorIDs {
		if id > 0 {
			monitorIDs[fmt.Sprintf("%d", id)] = struct{}{}
		}
	}
	serverIDs := map[string]struct{}{}
	for _, id := range block.ServerIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			serverIDs[id] = struct{}{}
		}
	}

	return &publicIncidentFilter{
		ScopePageID:  scopePageID,
		Limit:        limit,
		ShowResolved: block.ShowResolved,
		SourceTypes:  sourceTypes,
		MonitorIDs:   monitorIDs,
		ServerIDs:    serverIDs,
	}
}

func matchPublicIncident(incident *models.Incident, filter *publicIncidentFilter) bool {
	if incident == nil || filter == nil || filter.Empty {
		return false
	}
	if !filter.ShowResolved && incident.Status != "active" {
		return false
	}
	if len(filter.SourceTypes) > 0 {
		if _, ok := filter.SourceTypes[incident.SourceType]; !ok {
			return false
		}
	}
	if incident.SourceType == "maintenance" {
		pageIDs := parseIncidentPageIDs(incident.PageIDs)
		if len(pageIDs) == 0 {
			return true
		}
		for _, id := range pageIDs {
			if id == filter.ScopePageID {
				return true
			}
		}
		return false
	}
	if incident.SourceType == "service_monitor" && len(filter.MonitorIDs) > 0 {
		if _, ok := filter.MonitorIDs[incident.SourceID]; !ok {
			return false
		}
	}
	if incident.SourceType == "server" && len(filter.ServerIDs) > 0 {
		if _, ok := filter.ServerIDs[incident.SourceID]; !ok {
			return false
		}
	}
	return true
}
