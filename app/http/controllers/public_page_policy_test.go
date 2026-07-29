package controllers

import "testing"

func TestPublicPagePolicyResolveBuildsPageAndIncidentScope(t *testing.T) {
	cfg := PublicPagesConfigV1{
		Version:                1,
		RefreshIntervalSeconds: 30,
		Pages: []PublicPageV1{{
			ID:   "status",
			Path: "/public/status",
			Blocks: []PublicPageBlockV1{
				mustBlock("serviceStatus", PublicBlockServiceStatusV1{MonitorIDs: []uint{11}}),
			},
		}},
	}
	ensurePublicIncidentsSeparated(&cfg)
	page, err := resolveBoundPageByPath(cfg, "/public/status")
	if err != nil {
		t.Fatalf("resolve page: %v", err)
	}
	filter := buildPublicIncidentFilter(page)
	if filter.Empty {
		t.Fatal("a status page must receive the compatibility incident block")
	}
	if _, ok := filter.MonitorIDs["11"]; !ok {
		t.Fatalf("monitor scope was not inherited: %#v", filter.MonitorIDs)
	}
}

func TestPublicPagePolicyRejectsUnknownPath(t *testing.T) {
	cfg := PublicPagesConfigV1{Pages: []PublicPageV1{{ID: "home", Path: "/public"}}}
	if _, err := resolveBoundPageByPath(cfg, "/public/missing"); err == nil {
		t.Fatal("unknown page path must be rejected")
	}
}
