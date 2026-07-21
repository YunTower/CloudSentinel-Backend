package controllers

import (
	"encoding/json"
	"errors"
	"strings"

	"goravel/app/repositories"

	"github.com/goravel/framework/facades"
)

const publicDisplaySettingKeyV1 = "public_display_v1"

const (
	publicDisplayServerFilterModeAll       = "all"
	publicDisplayServerFilterModeAllowList = "allowList"
)

type PublicDisplayOverviewV1 struct {
	DefaultViewMode     string `json:"defaultViewMode"`
	AllowViewModeSwitch bool   `json:"allowViewModeSwitch"`
	DefaultGroupBy      string `json:"defaultGroupBy"`
	AllowGroupBySwitch  bool   `json:"allowGroupBySwitch"`
}

type PublicDisplayServerFilterV1 struct {
	Mode           string   `json:"mode"`
	AllowServerIDs []string `json:"allowServerIds"`
	AllowGroupIDs  []uint   `json:"allowGroupIds"`
}

type PublicDisplayFieldsV1 struct {
	ShowLocation     bool `json:"showLocation"`
	ShowOS           bool `json:"showOS"`
	ShowArchitecture bool `json:"showArchitecture"`
	ShowCores        bool `json:"showCores"`
	ShowNetworkIO    bool `json:"showNetworkIO"`
	ShowBilling      bool `json:"showBilling"`
	ShowTraffic      bool `json:"showTraffic"`
}

type PublicDisplayAnnouncementV1 struct {
	Enabled   bool   `json:"enabled"`
	Markdown  string `json:"markdown"`
	Placement string `json:"placement"`
}

type PublicDisplayConfigV1 struct {
	Version      int                         `json:"version"`
	Enabled      bool                        `json:"enabled"`
	Overview     PublicDisplayOverviewV1     `json:"overview"`
	ServerFilter PublicDisplayServerFilterV1 `json:"serverFilter"`
	Fields       PublicDisplayFieldsV1       `json:"fields"`
	Announcement PublicDisplayAnnouncementV1 `json:"announcement"`
}

func defaultPublicDisplayConfigV1() PublicDisplayConfigV1 {
	return PublicDisplayConfigV1{
		Version: 1,
		Enabled: true,
		Overview: PublicDisplayOverviewV1{
			DefaultViewMode:     "card",
			AllowViewModeSwitch: true,
			DefaultGroupBy:      "none",
			AllowGroupBySwitch:  true,
		},
		ServerFilter: PublicDisplayServerFilterV1{
			Mode:           publicDisplayServerFilterModeAll,
			AllowServerIDs: []string{},
			AllowGroupIDs:  []uint{},
		},
		Fields: PublicDisplayFieldsV1{
			ShowLocation:     true,
			ShowOS:           true,
			ShowArchitecture: true,
			ShowCores:        true,
			ShowNetworkIO:    true,
			ShowBilling:      true,
			ShowTraffic:      true,
		},
		Announcement: PublicDisplayAnnouncementV1{
			Enabled:   false,
			Markdown:  "",
			Placement: "overview_top",
		},
	}
}

func loadPublicDisplayConfigV1() PublicDisplayConfigV1 {
	repo := repositories.GetSystemSettingRepository()

	def := defaultPublicDisplayConfigV1()
	raw := repo.GetValue(publicDisplaySettingKeyV1, "")
	if strings.TrimSpace(raw) == "" {
		normalizePublicDisplayConfigV1(&def)
		return def
	}

	// 深度合并：让新增字段可以使用默认值（避免旧配置缺字段导致 bool 变为 false）
	var storedMap map[string]any
	if err := json.Unmarshal([]byte(raw), &storedMap); err != nil {
		normalizePublicDisplayConfigV1(&def)
		return def
	}

	var defMap map[string]any
	defBytes, _ := json.Marshal(def)
	_ = json.Unmarshal(defBytes, &defMap)

	merged := deepMergeMap(defMap, storedMap)
	mergedBytes, _ := json.Marshal(merged)

	cfg := def
	if err := json.Unmarshal(mergedBytes, &cfg); err != nil {
		normalizePublicDisplayConfigV1(&def)
		return def
	}
	normalizePublicDisplayConfigV1(&cfg)
	return cfg
}

func deepMergeMap(dst map[string]any, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, v := range src {
		if vMap, ok := v.(map[string]any); ok {
			if dstMap, ok := dst[k].(map[string]any); ok {
				dst[k] = deepMergeMap(dstMap, vMap)
				continue
			}
		}
		dst[k] = v
	}
	return dst
}

func normalizePublicDisplayConfigV1(cfg *PublicDisplayConfigV1) {
	if cfg == nil {
		return
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Overview.DefaultViewMode == "" {
		cfg.Overview.DefaultViewMode = "card"
	}
	if cfg.Overview.DefaultGroupBy == "" {
		cfg.Overview.DefaultGroupBy = "none"
	}
	if cfg.ServerFilter.Mode == "" {
		cfg.ServerFilter.Mode = publicDisplayServerFilterModeAll
	}
	if cfg.Announcement.Placement == "" {
		cfg.Announcement.Placement = "overview_top"
	}
	// 规范化 server ids：去空格、去空项、去重（保序）
	if len(cfg.ServerFilter.AllowServerIDs) > 0 {
		seen := make(map[string]struct{}, len(cfg.ServerFilter.AllowServerIDs))
		out := make([]string, 0, len(cfg.ServerFilter.AllowServerIDs))
		for _, id := range cfg.ServerFilter.AllowServerIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		cfg.ServerFilter.AllowServerIDs = out
	}
	// 规范化 group ids：去重（保序）
	if len(cfg.ServerFilter.AllowGroupIDs) > 0 {
		seen := make(map[uint]struct{}, len(cfg.ServerFilter.AllowGroupIDs))
		out := make([]uint, 0, len(cfg.ServerFilter.AllowGroupIDs))
		for _, id := range cfg.ServerFilter.AllowGroupIDs {
			if id == 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		cfg.ServerFilter.AllowGroupIDs = out
	}
}

func validatePublicDisplayConfigV1(cfg *PublicDisplayConfigV1) error {
	if cfg == nil {
		return errors.New("配置不能为空")
	}
	if cfg.Version != 1 {
		return errors.New("仅支持 version=1")
	}
	switch cfg.Overview.DefaultViewMode {
	case "card", "table":
	default:
		return errors.New("overview.defaultViewMode 只能是 card 或 table")
	}
	switch cfg.Overview.DefaultGroupBy {
	case "none", "status", "os", "location":
	default:
		// 兼容后续扩展（例如 group:123），暂时不做强校验阻断
		if !strings.HasPrefix(cfg.Overview.DefaultGroupBy, "group:") {
			return errors.New("overview.defaultGroupBy 值不合法")
		}
	}
	switch cfg.ServerFilter.Mode {
	case publicDisplayServerFilterModeAll, publicDisplayServerFilterModeAllowList:
	default:
		return errors.New("serverFilter.mode 只能是 all 或 allowList")
	}
	if len(cfg.Announcement.Markdown) > 20000 {
		return errors.New("announcement.markdown 过长（最大 20000 字符）")
	}
	if len(cfg.ServerFilter.AllowServerIDs) > 1000 {
		return errors.New("serverFilter.allowServerIds 过多（最大 1000）")
	}
	if len(cfg.ServerFilter.AllowGroupIDs) > 500 {
		return errors.New("serverFilter.allowGroupIds 过多（最大 500）")
	}
	return nil
}

func savePublicDisplayConfigV1(cfg PublicDisplayConfigV1) error {
	repo := repositories.GetSystemSettingRepository()
	return repo.SetJSON(publicDisplaySettingKeyV1, cfg)
}

func guestAllowedServerIDsFromConfigV1(cfg PublicDisplayConfigV1) (restrict bool, allowedIDs []string) {
	if !cfg.Enabled {
		return true, []string{}
	}
	if cfg.ServerFilter.Mode != publicDisplayServerFilterModeAllowList {
		return false, nil
	}

	set := make(map[string]struct{}, len(cfg.ServerFilter.AllowServerIDs)+64)
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		set[id] = struct{}{}
	}
	for _, id := range cfg.ServerFilter.AllowServerIDs {
		add(id)
	}

	if len(cfg.ServerFilter.AllowGroupIDs) > 0 {
		var rows []map[string]interface{}
		groupIDs := make([]interface{}, 0, len(cfg.ServerFilter.AllowGroupIDs))
		for _, gid := range cfg.ServerFilter.AllowGroupIDs {
			groupIDs = append(groupIDs, gid)
		}

		// 仅需要 id 列
		err := facades.Orm().Query().Table("servers").
			Select("id").
			WhereIn("group_id", groupIDs).
			Get(&rows)
		if err == nil && len(rows) > 0 {
			for _, row := range rows {
				if id, ok := row["id"].(string); ok {
					add(id)
				}
			}
		}
	}

	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return true, out
}

func isServerAllowedForGuestV1(cfg PublicDisplayConfigV1, serverID string, groupID *uint) bool {
	if !cfg.Enabled {
		return false
	}
	if cfg.ServerFilter.Mode != publicDisplayServerFilterModeAllowList {
		return true
	}
	for _, id := range cfg.ServerFilter.AllowServerIDs {
		if id == serverID {
			return true
		}
	}
	if groupID != nil {
		for _, gid := range cfg.ServerFilter.AllowGroupIDs {
			if gid == *groupID {
				return true
			}
		}
	}
	return false
}

// publicDisplayPublicPayloadV1 返回公开侧可下发的配置子集（避免泄露 allowlist 细节）
func publicDisplayPublicPayloadV1(cfg PublicDisplayConfigV1) map[string]any {
	return map[string]any{
		"version":      cfg.Version,
		"enabled":      cfg.Enabled,
		"overview":     cfg.Overview,
		"fields":       cfg.Fields,
		"announcement": cfg.Announcement,
		"serverFilter": map[string]any{
			"mode": cfg.ServerFilter.Mode,
		},
	}
}

func decodePublicDisplayConfigFromAny(input any) (PublicDisplayConfigV1, error) {
	// 允许前端传 JSON / form merged 的 map，统一走 marshal/unmarshal
	raw, err := json.Marshal(input)
	if err != nil {
		return PublicDisplayConfigV1{}, err
	}
	var cfg PublicDisplayConfigV1
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return PublicDisplayConfigV1{}, err
	}
	normalizePublicDisplayConfigV1(&cfg)
	return cfg, nil
}
