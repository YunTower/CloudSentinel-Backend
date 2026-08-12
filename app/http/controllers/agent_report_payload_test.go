package controllers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func compressedWrapper(t *testing.T, value interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return map[string]interface{}{"compressed": true, "compression": "gzip", "encoding": "base64", "payload": base64.StdEncoding.EncodeToString(buf.Bytes())}
}

func TestDecodeAgentReportPayloadLeavesPlainDataAndDecodesCompressedJSON(t *testing.T) {
	plain := map[string]interface{}{"cpu": 12.5}
	got, err := decodeAgentReportPayload(plain)
	if err != nil || got.(map[string]interface{})["cpu"] != 12.5 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	value := []interface{}{map[string]interface{}{"name": "disk", "used": float64(42)}}
	got, err = decodeAgentReportPayload(compressedWrapper(t, value))
	if err != nil {
		t.Fatal(err)
	}
	decoded := got.([]interface{})
	if decoded[0].(map[string]interface{})["name"] != "disk" {
		t.Fatalf("decoded=%v", decoded)
	}
}

func TestDecodeAgentReportPayloadRejectsMalformedAndOversizedData(t *testing.T) {
	for _, wrapper := range []map[string]interface{}{
		{"compressed": true},
		{"compressed": true, "compression": "gzip", "encoding": "base64", "payload": "%%%"},
		{"compressed": true, "compression": "gzip", "encoding": "base64", "payload": base64.StdEncoding.EncodeToString([]byte("not gzip"))},
	} {
		if _, err := decodeAgentReportPayload(wrapper); err == nil {
			t.Fatalf("expected error for %v", wrapper)
		}
	}
	big := compressedWrapper(t, strings.Repeat("x", maxAgentReportDecompressedBytes+1))
	if _, err := decodeAgentReportPayload(big); err == nil || !strings.Contains(err.Error(), "超过大小限制") {
		t.Fatalf("err=%v", err)
	}
}
