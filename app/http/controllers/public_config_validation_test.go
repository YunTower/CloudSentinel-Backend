package controllers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizePublicPagesTrimsFieldsInitializesSlicesAndClampsRefresh(t *testing.T) {
	cfg := PublicPagesConfigV1{RefreshIntervalSeconds: 1, Pages: []PublicPageV1{{
		ID: " home ", Path: " /public ", Title: " 状态 ", BrandName: " 品牌 ", LogoURL: " /logo ", AccentColor: " #fff ",
		Blocks: []PublicPageBlockV1{{Type: " markdown ", Data: json.RawMessage(`{"markdown":"ok"}`)}},
	}}}
	normalizePublicPagesConfigV1(&cfg)
	if cfg.Version != 1 || cfg.RefreshIntervalSeconds != 5 || cfg.Pages[0].ID != "home" || cfg.Pages[0].Path != "/public" || cfg.Pages[0].Blocks[0].Type != "markdown" {
		t.Fatalf("cfg=%+v", cfg)
	}
	cfg.RefreshIntervalSeconds = 9999
	normalizePublicPagesConfigV1(&cfg)
	if cfg.RefreshIntervalSeconds != 3600 {
		t.Fatal("refresh not clamped")
	}
}

func TestValidateDefaultPublicPagesAndRejectsInvalidContracts(t *testing.T) {
	valid := defaultPublicPagesConfigV1()
	normalizePublicPagesConfigV1(&valid)
	if err := validatePublicPagesConfigV1(&valid); err != nil {
		t.Fatalf("default invalid: %v", err)
	}
	for name, mutate := range map[string]func(*PublicPagesConfigV1){
		"version":        func(c *PublicPagesConfigV1) { c.Version = 2 },
		"empty pages":    func(c *PublicPagesConfigV1) { c.Pages = nil },
		"bad path":       func(c *PublicPagesConfigV1) { c.Pages[0].Path = "/admin" },
		"duplicate id":   func(c *PublicPagesConfigV1) { c.Pages = append(c.Pages, c.Pages[0]); c.Pages[1].Path = "/public/two" },
		"duplicate path": func(c *PublicPagesConfigV1) { c.Pages = append(c.Pages, c.Pages[0]); c.Pages[1].ID = "two" },
		"bad color":      func(c *PublicPagesConfigV1) { c.Pages[0].AccentColor = "red" },
		"unknown block": func(c *PublicPagesConfigV1) {
			c.Pages[0].Blocks = []PublicPageBlockV1{mustBlock("unknown", map[string]any{})}
		},
		"bad link": func(c *PublicPagesConfigV1) {
			c.Pages[0].Blocks = []PublicPageBlockV1{mustBlock("links", PublicBlockLinksV1{Links: []PublicBlockLinkV1{{Href: "/x"}}})}
		},
		"bad incident source": func(c *PublicPagesConfigV1) {
			c.Pages[0].Blocks = []PublicPageBlockV1{mustBlock("incidents", PublicBlockIncidentsV1{SourceTypes: []string{"invalid"}})}
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := defaultPublicPagesConfigV1()
			mutate(&cfg)
			if err := validatePublicPagesConfigV1(&cfg); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestPublicHexColorAcceptsOnlyThreeOrSixHexDigits(t *testing.T) {
	for _, value := range []string{"#fff", "#18a058", "#ABCDEF"} {
		if !isValidPublicHexColor(value) {
			t.Errorf("%s rejected", value)
		}
	}
	for _, value := range []string{"fff", "#ff", "#ffff", "#ggg", "#1234567"} {
		if isValidPublicHexColor(value) {
			t.Errorf("%s accepted", value)
		}
	}
}

func TestPublicDisplayNormalizeValidateAndGuestRules(t *testing.T) {
	cfg := PublicDisplayConfigV1{Enabled: true, ServerFilter: PublicDisplayServerFilterV1{Mode: "allowList", AllowServerIDs: []string{" s1 ", "", "s1", "s2"}, AllowGroupIDs: []uint{2, 0, 2, 3}}}
	normalizePublicDisplayConfigV1(&cfg)
	if cfg.Version != 1 || cfg.Overview.DefaultViewMode != "card" || cfg.Overview.DefaultGroupBy != "none" || cfg.Announcement.Placement != "overview_top" {
		t.Fatalf("defaults=%+v", cfg)
	}
	if strings.Join(cfg.ServerFilter.AllowServerIDs, ",") != "s1,s2" || len(cfg.ServerFilter.AllowGroupIDs) != 2 {
		t.Fatalf("filter=%+v", cfg.ServerFilter)
	}
	if err := validatePublicDisplayConfigV1(&cfg); err != nil {
		t.Fatal(err)
	}
	gid := uint(3)
	if !isServerAllowedForGuestV1(cfg, "other", &gid) || !isServerAllowedForGuestV1(cfg, "s1", nil) || isServerAllowedForGuestV1(cfg, "other", nil) {
		t.Fatal("allowlist mismatch")
	}
	cfg.Enabled = false
	if isServerAllowedForGuestV1(cfg, "s1", nil) {
		t.Fatal("disabled display allowed server")
	}
	restrict, ids := guestAllowedServerIDsFromConfigV1(cfg)
	if !restrict || len(ids) != 0 {
		t.Fatalf("restrict=%v ids=%v", restrict, ids)
	}
}

func TestPublicDisplayValidationRejectsInvalidEnumsAndOversizedMarkdown(t *testing.T) {
	for name, mutate := range map[string]func(*PublicDisplayConfigV1){
		"version":  func(c *PublicDisplayConfigV1) { c.Version = 2 },
		"view":     func(c *PublicDisplayConfigV1) { c.Overview.DefaultViewMode = "grid" },
		"group":    func(c *PublicDisplayConfigV1) { c.Overview.DefaultGroupBy = "bad" },
		"filter":   func(c *PublicDisplayConfigV1) { c.ServerFilter.Mode = "bad" },
		"markdown": func(c *PublicDisplayConfigV1) { c.Announcement.Markdown = strings.Repeat("x", 20001) },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := defaultPublicDisplayConfigV1()
			mutate(&cfg)
			if err := validatePublicDisplayConfigV1(&cfg); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	cfg, err := decodePublicDisplayConfigFromAny(map[string]any{"enabled": true, "serverFilter": map[string]any{"mode": "all"}})
	if err != nil || cfg.Version != 1 || !cfg.Enabled {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}
