package security

import (
	"context"
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

func TestSafeDialContextPinsValidatedResolution(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- struct{}{}
			_ = conn.Close()
		}
	}()

	previousLookup := lookupIPAddr
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "panel.example" {
			t.Fatalf("unexpected lookup host %q", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	t.Cleanup(func() { lookupIPAddr = previousLookup })

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := SafeDialContext(true)(context.Background(), "tcp", net.JoinHostPort("panel.example", port))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("did not connect to the validated IP")
	}
}

func TestSafeDialContextRejectsBlockedResolvedIP(t *testing.T) {
	previousLookup := lookupIPAddr
	lookupIPAddr = func(_ context.Context, _ string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	t.Cleanup(func() { lookupIPAddr = previousLookup })

	if _, err := SafeDialContext(false)(context.Background(), "tcp", "panel.example:443"); err == nil || !strings.Contains(err.Error(), "禁止访问内网") {
		t.Fatalf("err=%v", err)
	}
}

func TestSafeDialContextRejectsReboundPrivateAddress(t *testing.T) {
	previousLookup := lookupIPAddr
	lookups := 0
	lookupIPAddr = func(_ context.Context, _ string) ([]net.IPAddr, error) {
		lookups++
		if lookups == 1 {
			return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	t.Cleanup(func() { lookupIPAddr = previousLookup })

	if err := ValidateHostForOutboundRequest("panel.example", time.Second, false); err != nil {
		t.Fatalf("initial public resolution should pass: %v", err)
	}
	if _, err := SafeDialContext(false)(context.Background(), "tcp", "panel.example:443"); err == nil || !strings.Contains(err.Error(), "禁止访问内网") {
		t.Fatalf("rebound private resolution must be rejected, err=%v", err)
	}
}
