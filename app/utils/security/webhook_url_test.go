package security

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestParseAndValidateWebhookURLForConfigAllowsPublicHTTPAndStripsFragment(t *testing.T) {
	u, err := ParseAndValidateWebhookURLForConfig(" https://example.com/hooks?a=1#secret ")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Hostname() != "example.com" || u.Fragment != "" || u.RawQuery != "a=1" {
		t.Fatalf("url=%s", u)
	}
}

func TestParseAndValidateWebhookURLForConfigRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"", "ftp://example.com/x", "https:///missing-host", "https://user:pass@example.com/x",
		"http://localhost/hook", "http://localhost./hook", "http://127.0.0.1/hook",
		"http://10.1.2.3/hook", "http://169.254.169.254/latest/meta-data", "http://[::1]/hook",
	} {
		if _, err := ParseAndValidateWebhookURLForConfig(raw); err == nil {
			t.Errorf("expected rejection for %q", raw)
		}
	}
}

func TestBlockedIPClassificationCoversReservedNetworks(t *testing.T) {
	for _, raw := range []string{"0.0.0.0", "10.0.0.1", "127.0.0.1", "169.254.1.1", "192.168.1.1", "100.64.0.1", "::1", "fc00::1", "fe80::1", "2001:db8::1"} {
		if !isBlockedIP(net.ParseIP(raw)) {
			t.Errorf("%s should be blocked", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if isBlockedIP(net.ParseIP(raw)) {
			t.Errorf("%s should be public", raw)
		}
	}
	if !isBlockedIP(nil) {
		t.Fatal("nil IP should be blocked")
	}
}

func TestResolveAndValidateWebhookURLForRequestAcceptsPublicLiteralWithoutDNS(t *testing.T) {
	u, ips, err := ResolveAndValidateWebhookURLForRequest("https://8.8.8.8/hook", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if u.Hostname() != "8.8.8.8" || len(ips) != 1 || !ips[0].Equal(net.ParseIP("8.8.8.8")) {
		t.Fatalf("url=%s ips=%v", u, ips)
	}
}

func TestResolveAndValidateWebhookURLForRequestReportsResolutionFailure(t *testing.T) {
	_, _, err := ResolveAndValidateWebhookURLForRequest("https://this-host-must-not-exist.invalid/hook", 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "解析失败") {
		t.Fatalf("err=%v", err)
	}
}
