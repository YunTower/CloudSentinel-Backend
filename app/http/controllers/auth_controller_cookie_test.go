package controllers

import "testing"

func TestNormalizeCookieSameSite(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "strict lower", input: "strict", expected: "strict"},
		{name: "strict upper", input: "STRICT", expected: "strict"},
		{name: "none mixed", input: "None", expected: "none"},
		{name: "lax lower", input: "lax", expected: "lax"},
		{name: "invalid fallback", input: "weird", expected: "lax"},
		{name: "empty fallback", input: "", expected: "lax"},
	}

	for _, tc := range cases {
		if got := normalizeCookieSameSite(tc.input); got != tc.expected {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.expected, got)
		}
	}
}

func TestBuildAuthCookieUsesConfig(t *testing.T) {
	config := authCookieConfig{
		Path:     "/admin",
		Domain:   "example.com",
		Secure:   false,
		SameSite: "lax",
	}

	cookie := buildAuthCookie("auth", "token", 3600, true, config)
	if cookie.Name != "auth" {
		t.Fatalf("expected cookie name auth, got %q", cookie.Name)
	}
	if cookie.Value != "token" {
		t.Fatalf("expected cookie value token, got %q", cookie.Value)
	}
	if cookie.Path != config.Path {
		t.Fatalf("expected cookie path %q, got %q", config.Path, cookie.Path)
	}
	if cookie.Domain != config.Domain {
		t.Fatalf("expected cookie domain %q, got %q", config.Domain, cookie.Domain)
	}
	if cookie.Secure != config.Secure {
		t.Fatalf("expected cookie secure %v, got %v", config.Secure, cookie.Secure)
	}
	if cookie.HttpOnly != true {
		t.Fatalf("expected cookie httpOnly true, got %v", cookie.HttpOnly)
	}
	if cookie.SameSite != config.SameSite {
		t.Fatalf("expected cookie sameSite %q, got %q", config.SameSite, cookie.SameSite)
	}
	if cookie.MaxAge != 3600 {
		t.Fatalf("expected cookie maxAge 3600, got %d", cookie.MaxAge)
	}
}

func TestBuildAuthCookieClearingKeepsConfigDimensions(t *testing.T) {
	config := authCookieConfig{
		Path:     "/",
		Domain:   "",
		Secure:   true,
		SameSite: "strict",
	}

	cookie := buildAuthCookie("auth", "", -1, false, config)
	if cookie.Path != config.Path {
		t.Fatalf("expected cookie path %q, got %q", config.Path, cookie.Path)
	}
	if cookie.Domain != config.Domain {
		t.Fatalf("expected cookie domain %q, got %q", config.Domain, cookie.Domain)
	}
	if cookie.Secure != config.Secure {
		t.Fatalf("expected cookie secure %v, got %v", config.Secure, cookie.Secure)
	}
	if cookie.SameSite != config.SameSite {
		t.Fatalf("expected cookie sameSite %q, got %q", config.SameSite, cookie.SameSite)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("expected clearing cookie maxAge -1, got %d", cookie.MaxAge)
	}
	if cookie.HttpOnly != false {
		t.Fatalf("expected csrf-style cookie httpOnly false, got %v", cookie.HttpOnly)
	}
}
