package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAggregateProbeResultsCoversUpDownAndPartialFailure(t *testing.T) {
	status, elapsed, err := aggregateProbeResults([]probeCheckResult{{probeID: "a", status: "up", responseTime: 10}, {probeID: "b", status: "up", responseTime: 20}})
	if status != "up" || elapsed != 20 || err != nil {
		t.Fatalf("status=%s elapsed=%d err=%v", status, elapsed, err)
	}
	status, elapsed, err = aggregateProbeResults([]probeCheckResult{{probeID: "a", status: "down", responseTime: 15, err: errors.New("timeout")}})
	if status != "down" || elapsed != 15 || err == nil || !strings.Contains(err.Error(), "a: timeout") {
		t.Fatalf("status=%s elapsed=%d err=%v", status, elapsed, err)
	}
	status, _, err = aggregateProbeResults([]probeCheckResult{{probeID: "a", status: "up"}, {probeID: "b", status: "slow"}, {probeID: "c", status: "down"}})
	if status != "slow" || err == nil || !strings.Contains(err.Error(), "up=1 slow=1 down=1") {
		t.Fatalf("status=%s err=%v", status, err)
	}
}

func TestAggregateHelpersAndProbeNormalization(t *testing.T) {
	if got := joinAggregateErrors(nil, "fallback").Error(); got != "fallback" {
		t.Fatalf("got=%q", got)
	}
	if got := joinAggregateErrors([]string{"a", "b"}, "fallback").Error(); got != "a; b" {
		t.Fatalf("got=%q", got)
	}
	if normalizeProbeType("agent") != "agent" || normalizeProbeType("panel") != "panel" || normalizeProbeType("bad") != "panel" {
		t.Fatal("probe normalization mismatch")
	}
	name, location := resolveProbeMetadata("panel", "anything")
	if name != "Panel" || location != "本地面板" {
		t.Fatalf("%s %s", name, location)
	}
	name, location = resolveProbeMetadata("agent", "unknown")
	if name != "unknown" || location != "" {
		t.Fatalf("%s %s", name, location)
	}
}

func TestHTTPMethodHeadersAndTargetParsing(t *testing.T) {
	if normalizeRequestMethod(" post ") != "POST" || normalizeRequestMethod("trace") != "GET" {
		t.Fatal("method normalization mismatch")
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if !methodAllowsBody(method) {
			t.Errorf("%s should allow body", method)
		}
	}
	if methodAllowsBody("GET") {
		t.Fatal("GET must not allow body")
	}
	headers, err := parseHTTPHeaders(`{"Authorization":"Bearer x","X-Test":"1"}`)
	if err != nil || headers["X-Test"] != "1" {
		t.Fatalf("headers=%v err=%v", headers, err)
	}
	if _, err := parseHTTPHeaders(`[]`); err == nil {
		t.Fatal("array headers should fail")
	}
	if _, err := parseHTTPHeaders(`{" ":"x"}`); err == nil {
		t.Fatal("blank header should fail")
	}
	if headers, err := parseHTTPHeaders(""); err != nil || len(headers) != 0 {
		t.Fatalf("headers=%v err=%v", headers, err)
	}

	tests := []struct {
		target      string
		defaultPort int
		host        string
		port        int
	}{
		{"https://example.com:8443/path", 443, "example.com", 8443},
		{"example.com:53", 80, "example.com", 53}, {"example.com", 443, "example.com", 443},
		{"[2001:db8::1]:443", 80, "2001:db8::1", 443},
	}
	for _, tc := range tests {
		host, port, err := splitMonitorTarget(tc.target, tc.defaultPort)
		if err != nil || host != tc.host || port != tc.port {
			t.Errorf("%s => %s:%d err=%v", tc.target, host, port, err)
		}
	}
	if _, _, err := splitMonitorTarget("", 80); err == nil {
		t.Fatal("empty target should fail")
	}
	if _, _, err := splitMonitorTarget("https:///missing", 443); err == nil {
		t.Fatal("missing host should fail")
	}
}

func TestCheckHTTPValidatesMethodBodyHeadersStatusAndExpectedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Test") != "yes" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("service healthy"))
	}))
	defer server.Close()
	_, err := checkHTTP(server.URL, 2, http.StatusCreated, "healthy", "post", `{"X-Test":"yes"}`, `{"ping":true}`, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkHTTP(server.URL, 2, http.StatusOK, "", "POST", `{"X-Test":"yes"}`, `{}`, false); err == nil {
		t.Fatal("status mismatch should fail")
	}
	if _, err := checkHTTP(server.URL, 2, http.StatusCreated, "missing", "POST", `{"X-Test":"yes"}`, `{}`, false); err == nil {
		t.Fatal("body mismatch should fail")
	}
}

func TestCheckHTTPRejectsServerErrorsByDefaultAndMissingTLSCertificate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusServiceUnavailable) }))
	defer server.Close()
	if _, err := checkHTTP(server.URL, 2, 0, "", "GET", "", "", false); err == nil {
		t.Fatal("5xx should fail")
	}
	if _, err := checkHTTP(server.URL, 2, 0, "", "GET", "", "", true); err == nil {
		t.Fatal("plain HTTP should have no TLS certificate")
	}
	if extractLeafCertificate(nil) != nil {
		t.Fatal("nil response should have no certificate")
	}
}
