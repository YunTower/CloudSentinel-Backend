package controllers

import (
	"testing"
	"time"

	"goravel/app/services"
)

func TestParseExpireTimeSupportsDocumentedFormats(t *testing.T) {
	for _, raw := range []string{"2026-08-12T12:34:56.123Z", "2026-08-12T12:34:56Z", "2026-08-12 12:34:56", "2026-08-12"} {
		if got := parseExpireTime(raw); got == nil {
			t.Errorf("failed to parse %s", raw)
		}
	}
	if parseExpireTime("not-a-date") != nil {
		t.Fatal("invalid date should return nil")
	}
	now := time.Now().Add(-time.Hour)
	if got := services.CalculateUptime(&now); got == "0分" {
		t.Fatalf("uptime=%q", got)
	}
}

func TestNormalizeTrafficSettingsAndResponseCycle(t *testing.T) {
	days := 15
	tests := []struct {
		limitType, cycle    string
		days                *int
		wantType, wantCycle string
		wantDays            *int
	}{
		{"", "monthly", &days, "periodic", "monthly", nil},
		{"", "custom", &days, "periodic", "custom", &days},
		{"", "unlimited", &days, "permanent", "unlimited", nil},
		{"permanent", "", &days, "permanent", "unlimited", nil},
		{"periodic", "legacy", &days, "periodic", "legacy", &days},
	}
	for _, tc := range tests {
		typ, cycle, gotDays := normalizeTrafficSettings(tc.limitType, tc.cycle, tc.days)
		if typ != tc.wantType || cycle != tc.wantCycle || (gotDays == nil) != (tc.wantDays == nil) {
			t.Errorf("%q %q => %q %q %v", tc.limitType, tc.cycle, typ, cycle, gotDays)
		}
	}
	if resolveTrafficCycleForResponse("permanent", "") != "unlimited" || resolveTrafficCycleForResponse("periodic", "monthly") != "monthly" || resolveTrafficCycleForResponse("periodic", "") != "" {
		t.Fatal("response cycle mismatch")
	}
}

func TestMaskAgentKeyNeverReturnsFullSecret(t *testing.T) {
	if got := maskAgentKey("12345678"); got != "********" {
		t.Fatalf("got=%q", got)
	}
	if got := maskAgentKey("abcd12345678wxyz"); got != "abcd********wxyz" {
		t.Fatalf("got=%q", got)
	}
}
