package middleware

import "testing"

func TestOriginSet_SplitsAndTrims(t *testing.T) {
	origins := originSet("https://a.com, https://b.com")
	if len(origins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(origins))
	}
	if _, ok := origins["https://a.com"]; !ok {
		t.Fatalf("expected origin https://a.com to be present")
	}
	if _, ok := origins["https://b.com"]; !ok {
		t.Fatalf("expected origin https://b.com to be present")
	}
}

func TestOriginSet_IgnoresEmptyItems(t *testing.T) {
	origins := originSet("https://a.com,, https://b.com, ")
	if len(origins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(origins))
	}
	if _, ok := origins["https://a.com"]; !ok {
		t.Fatalf("expected origin https://a.com to be present")
	}
	if _, ok := origins["https://b.com"]; !ok {
		t.Fatalf("expected origin https://b.com to be present")
	}
}

func TestOriginSet_EmptyRawReturnsEmptyMap(t *testing.T) {
	origins := originSet("")
	if len(origins) != 0 {
		t.Fatalf("expected 0 origins, got %d", len(origins))
	}
}

func TestIsPublicAPIPath(t *testing.T) {
	cases := []struct {
		path     string
		expected bool
	}{
		{path: "/api/settings/public", expected: true},
		{path: "api/settings/public", expected: true},
		{path: "/api/public/x", expected: true},
		{path: "api/public/x", expected: true},
		{path: "/api/private/x", expected: false},
		{path: "/api/public", expected: false},
	}

	for _, tc := range cases {
		if got := isPublicAPIPath(tc.path); got != tc.expected {
			t.Fatalf("isPublicAPIPath(%q) expected %v, got %v", tc.path, tc.expected, got)
		}
	}
}

