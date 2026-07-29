package controllers

import (
	"encoding/json"
	"errors"
	"strings"
)

const publicPagesSettingKeyV1 = "public_pages_v1"

type PublicPagesConfigV1 struct {
	Version                int            `json:"version"`
	RefreshIntervalSeconds int            `json:"refreshIntervalSeconds"`
	Pages                  []PublicPageV1 `json:"pages"`
}

type PublicPageV1 struct {
	ID          string              `json:"id"`
	Path        string              `json:"path"`
	Title       string              `json:"title"`
	BrandName   string              `json:"brandName,omitempty"`
	LogoURL     string              `json:"logoUrl,omitempty"`
	AccentColor string              `json:"accentColor,omitempty"`
	Blocks      []PublicPageBlockV1 `json:"blocks"`
}

type PublicPageBlockV1 struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type PublicBlockHeroV1 struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Badge    string `json:"badge"`
}

type PublicBlockMarkdownV1 struct {
	Markdown string `json:"markdown"`
}

type PublicBlockStatsV1 struct {
	Items []string `json:"items"`
}

type PublicBlockServerListV1 struct {
	View        string `json:"view"`    // card | table
	GroupBy     string `json:"groupBy"` // none | status | location | os
	Limit       int    `json:"limit"`   // 0 = unlimited
	ShowToolbar bool   `json:"showToolbar"`
}

type PublicBlockServiceStatusV1 struct {
	MonitorIDs []uint `json:"monitorIds"`
	GroupBy    string `json:"groupBy"` // none | group
	Limit      int    `json:"limit"`   // 0 = unlimited
	ShowUptime bool   `json:"showUptime"`
}

type PublicBlockIncidentsV1 struct {
	Limit        int      `json:"limit"`        // 0 = default
	ShowResolved bool     `json:"showResolved"` // include resolved incidents
	SourceTypes  []string `json:"sourceTypes"`  // empty = all
	MonitorIDs   []uint   `json:"monitorIds"`   // empty = all service_monitor incidents
	ServerIDs    []string `json:"serverIds"`    // empty = all server incidents
}

type PublicBlockLinksV1 struct {
	Links []PublicBlockLinkV1 `json:"links"`
}

type PublicBlockLinkV1 struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

func defaultPublicPagesConfigV1() PublicPagesConfigV1 {
	return PublicPagesConfigV1{
		Version:                1,
		RefreshIntervalSeconds: 30,
		Pages: []PublicPageV1{
			{
				ID:          "home",
				Path:        "/public",
				Title:       "状态",
				BrandName:   "CloudSentinel",
				AccentColor: "#18a058",
				Blocks: []PublicPageBlockV1{
					mustBlock("markdown", PublicBlockMarkdownV1{
						Markdown: "## 公告\n欢迎访问公开页面。\n\n- 本页内容由管理员配置\n- 指标为实时/准实时展示",
					}),
					mustBlock("serviceStatus", PublicBlockServiceStatusV1{
						MonitorIDs: []uint{},
						GroupBy:    "group",
						Limit:      0,
						ShowUptime: true,
					}),
					mustBlock("serverList", PublicBlockServerListV1{
						View:        "table",
						GroupBy:     "none",
						Limit:       0,
						ShowToolbar: false,
					}),
					mustBlock("links", PublicBlockLinksV1{
						Links: []PublicBlockLinkV1{
							{Label: "联系管理员", Href: "mailto:ops@example.com"},
						},
					}),
					mustBlock("incidents", PublicBlockIncidentsV1{
						Limit:        20,
						ShowResolved: true,
						SourceTypes:  []string{},
					}),
				},
			},
		},
	}
}

func mustBlock(t string, data any) PublicPageBlockV1 {
	raw, _ := json.Marshal(data)
	return PublicPageBlockV1{Type: t, Data: raw}
}

func loadPublicPagesConfigV1() PublicPagesConfigV1 {
	return getPublicPagePolicy().Load()
}

func normalizePublicPagesConfigV1(cfg *PublicPagesConfigV1) {
	if cfg == nil {
		return
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.RefreshIntervalSeconds <= 0 {
		cfg.RefreshIntervalSeconds = 30
	}
	if cfg.RefreshIntervalSeconds < 5 {
		cfg.RefreshIntervalSeconds = 5
	}
	if cfg.RefreshIntervalSeconds > 3600 {
		cfg.RefreshIntervalSeconds = 3600
	}
	if cfg.Pages == nil {
		cfg.Pages = []PublicPageV1{}
	}
	for i := range cfg.Pages {
		cfg.Pages[i].ID = strings.TrimSpace(cfg.Pages[i].ID)
		cfg.Pages[i].Path = strings.TrimSpace(cfg.Pages[i].Path)
		cfg.Pages[i].Title = strings.TrimSpace(cfg.Pages[i].Title)
		cfg.Pages[i].BrandName = strings.TrimSpace(cfg.Pages[i].BrandName)
		cfg.Pages[i].LogoURL = strings.TrimSpace(cfg.Pages[i].LogoURL)
		cfg.Pages[i].AccentColor = strings.TrimSpace(cfg.Pages[i].AccentColor)
		if cfg.Pages[i].Blocks == nil {
			cfg.Pages[i].Blocks = []PublicPageBlockV1{}
		}
		for j := range cfg.Pages[i].Blocks {
			cfg.Pages[i].Blocks[j].Type = strings.TrimSpace(cfg.Pages[i].Blocks[j].Type)
		}
	}
}

func validatePublicPagesConfigV1(cfg *PublicPagesConfigV1) error {
	if cfg == nil {
		return errors.New("配置不能为空")
	}
	if cfg.Version != 1 {
		return errors.New("仅支持 version=1")
	}
	if cfg.RefreshIntervalSeconds < 5 || cfg.RefreshIntervalSeconds > 3600 {
		return errors.New("refreshIntervalSeconds 必须在 5–3600 秒之间")
	}
	if len(cfg.Pages) == 0 {
		return errors.New("pages 不能为空")
	}
	if len(cfg.Pages) > 10 {
		return errors.New("pages 过多（最大 10）")
	}

	seenID := make(map[string]struct{}, len(cfg.Pages))
	seenPath := make(map[string]struct{}, len(cfg.Pages))

	for _, p := range cfg.Pages {
		if p.ID == "" {
			return errors.New("page.id 不能为空")
		}
		if p.Path == "" || !strings.HasPrefix(p.Path, "/public") {
			return errors.New("page.path 必须以 /public 开头")
		}
		if _, ok := seenID[p.ID]; ok {
			return errors.New("page.id 不能重复")
		}
		if _, ok := seenPath[p.Path]; ok {
			return errors.New("page.path 不能重复")
		}
		seenID[p.ID] = struct{}{}
		seenPath[p.Path] = struct{}{}

		if len(p.Blocks) == 0 {
			return errors.New("page.blocks 不能为空")
		}
		if len(p.BrandName) > 80 || len(p.LogoURL) > 2048 || len(p.AccentColor) > 32 {
			return errors.New("page 品牌字段过长")
		}
		if p.AccentColor != "" && !isValidPublicHexColor(p.AccentColor) {
			return errors.New("page.accentColor 必须是 #RGB 或 #RRGGBB")
		}
		if len(p.Blocks) > 50 {
			return errors.New("page.blocks 过多（最大 50）")
		}
		for _, b := range p.Blocks {
			if b.Type == "" {
				return errors.New("block.type 不能为空")
			}
			switch b.Type {
			case "hero":
				var d PublicBlockHeroV1
				if err := json.Unmarshal(b.Data, &d); err != nil {
					return errors.New("hero.data 格式错误")
				}
				if len(d.Title) > 200 || len(d.Subtitle) > 400 || len(d.Badge) > 40 {
					return errors.New("hero 字段过长")
				}
			case "markdown":
				var d PublicBlockMarkdownV1
				if err := json.Unmarshal(b.Data, &d); err != nil {
					return errors.New("markdown.data 格式错误")
				}
				if len(d.Markdown) > 20000 {
					return errors.New("markdown 过长（最大 20000 字符）")
				}
			case "stats":
				var d PublicBlockStatsV1
				if err := json.Unmarshal(b.Data, &d); err != nil {
					return errors.New("stats.data 格式错误")
				}
				if len(d.Items) > 20 {
					return errors.New("stats.items 过多（最大 20）")
				}
			case "serverList":
				var d PublicBlockServerListV1
				if err := json.Unmarshal(b.Data, &d); err != nil {
					return errors.New("serverList.data 格式错误")
				}
				if d.View == "" {
					d.View = "table"
				}
				if d.View != "card" && d.View != "table" {
					return errors.New("serverList.view 只能是 card 或 table")
				}
				if d.GroupBy != "" && d.GroupBy != "none" && d.GroupBy != "status" && d.GroupBy != "location" && d.GroupBy != "os" {
					return errors.New("serverList.groupBy 值不合法")
				}
				if d.Limit < 0 || d.Limit > 1000 {
					return errors.New("serverList.limit 范围为 0-1000")
				}
			case "serviceStatus":
				var d PublicBlockServiceStatusV1
				if err := json.Unmarshal(b.Data, &d); err != nil {
					return errors.New("serviceStatus.data 格式错误")
				}
				if len(d.MonitorIDs) > 500 {
					return errors.New("serviceStatus.monitorIds 过多（最大 500）")
				}
				if d.GroupBy != "" && d.GroupBy != "none" && d.GroupBy != "group" {
					return errors.New("serviceStatus.groupBy 值不合法")
				}
				if d.Limit < 0 || d.Limit > 500 {
					return errors.New("serviceStatus.limit 范围为 0-500")
				}
			case "incidents":
				var d PublicBlockIncidentsV1
				if err := json.Unmarshal(b.Data, &d); err != nil {
					return errors.New("incidents.data 格式错误")
				}
				if d.Limit < 0 || d.Limit > 50 {
					return errors.New("incidents.limit 范围为 0-50")
				}
				if len(d.SourceTypes) > 5 {
					return errors.New("incidents.sourceTypes 过多（最大 5）")
				}
				for _, sourceType := range d.SourceTypes {
					if sourceType != "server" && sourceType != "service_monitor" && sourceType != "maintenance" {
						return errors.New("incidents.sourceTypes 值不合法")
					}
				}
				if len(d.MonitorIDs) > 500 {
					return errors.New("incidents.monitorIds 过多（最大 500）")
				}
				if len(d.ServerIDs) > 500 {
					return errors.New("incidents.serverIds 过多（最大 500）")
				}
				for _, serverID := range d.ServerIDs {
					if strings.TrimSpace(serverID) == "" || len(serverID) > 100 {
						return errors.New("incidents.serverIds 值不合法")
					}
				}
			case "links":
				var d PublicBlockLinksV1
				if err := json.Unmarshal(b.Data, &d); err != nil {
					return errors.New("links.data 格式错误")
				}
				if len(d.Links) > 30 {
					return errors.New("links.links 过多（最大 30）")
				}
				for _, l := range d.Links {
					if strings.TrimSpace(l.Label) == "" || strings.TrimSpace(l.Href) == "" {
						return errors.New("links.link 的 label/href 不能为空")
					}
					if len(l.Label) > 60 || len(l.Href) > 2048 {
						return errors.New("links.link 字段过长")
					}
				}
			default:
				return errors.New("不支持的 block.type: " + b.Type)
			}
		}
	}

	return nil
}

func savePublicPagesConfigV1(cfg PublicPagesConfigV1) error {
	return getPublicPagePolicy().Save(cfg)
}

func isValidPublicHexColor(value string) bool {
	if len(value) != 4 && len(value) != 7 {
		return false
	}
	if value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}
