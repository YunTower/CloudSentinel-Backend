package controllers

import "testing"

func TestPublicSettingsBasePayloadOnlyContainsPanelTitle(t *testing.T) {
	payload := publicSettingsBasePayload("CloudSentinel")

	if len(payload) != 1 {
		t.Fatalf("expected exactly one base settings field, got %d", len(payload))
	}
	if payload["panel_title"] != "CloudSentinel" {
		t.Fatalf("unexpected panel_title: %v", payload["panel_title"])
	}
}

func TestPublicPageSettingsPayloadOnlyContainsPageRequirements(t *testing.T) {
	display := defaultPublicDisplayConfigV1()
	page := &PublicPageV1{ID: "home", Path: "/public", Title: "状态"}
	payload := publicPageSettingsPayload(display, 30, page)

	if len(payload) != 2 {
		t.Fatalf("expected exactly two routed settings fields, got %d", len(payload))
	}
	if _, ok := payload["panel_title"]; ok {
		t.Fatal("routed settings payload must not contain panel_title")
	}

	publicDisplay, ok := payload["public_display"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected public_display type: %T", payload["public_display"])
	}
	if len(publicDisplay) != 2 {
		t.Fatalf("expected only enabled and fields in public_display, got %d fields", len(publicDisplay))
	}
	if _, ok := publicDisplay["enabled"]; !ok {
		t.Fatal("public_display.enabled is missing")
	}
	if _, ok := publicDisplay["fields"]; !ok {
		t.Fatal("public_display.fields is missing")
	}

	publicPages, ok := payload["public_pages"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected public_pages type: %T", payload["public_pages"])
	}
	if len(publicPages) != 2 {
		t.Fatalf("expected only refreshIntervalSeconds and page in public_pages, got %d fields", len(publicPages))
	}
	if publicPages["refreshIntervalSeconds"] != 30 {
		t.Fatalf("unexpected refresh interval: %v", publicPages["refreshIntervalSeconds"])
	}
	if publicPages["page"] != page {
		t.Fatal("unexpected page payload")
	}
}

func TestResolveBoundPageByPath(t *testing.T) {
	cfg := PublicPagesConfigV1{
		Pages: []PublicPageV1{
			{ID: "home", Path: "/public", Title: "状态"},
			{ID: "team", Path: "/public/team", Title: "团队状态"},
		},
	}

	page, err := resolveBoundPageByPath(cfg, "/public/team/")
	if err != nil || page.ID != "team" {
		t.Fatalf("expected team page, got page=%v err=%v", page, err)
	}

	page, err = resolveBoundPageByPath(cfg, "/public/team/incidents")
	if err != nil || page.ID != "team" {
		t.Fatalf("expected team page for incidents route, got page=%v err=%v", page, err)
	}

	if _, err = resolveBoundPageByPath(cfg, "/public/missing"); err == nil {
		t.Fatal("expected missing public page to return an error")
	}
}
