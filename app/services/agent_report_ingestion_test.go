package services

import (
	"errors"
	"testing"
)

type recordingAgentDataSaver struct {
	metrics map[string]interface{}
}

func (s *recordingAgentDataSaver) SaveSystemInfo(string, map[string]interface{}) error { return nil }
func (s *recordingAgentDataSaver) SaveMetrics(_ string, data map[string]interface{}) error {
	s.metrics = data
	return nil
}
func (s *recordingAgentDataSaver) SaveMemoryInfo(string, map[string]interface{}) error  { return nil }
func (s *recordingAgentDataSaver) SaveDiskInfo(string, []interface{}) error             { return nil }
func (s *recordingAgentDataSaver) SaveDiskIO(string, map[string]interface{}) error      { return nil }
func (s *recordingAgentDataSaver) SaveNetworkInfo(string, map[string]interface{}) error { return nil }
func (s *recordingAgentDataSaver) SaveSwapInfo(string, map[string]interface{}) error    { return nil }
func (s *recordingAgentDataSaver) SaveProcessInfo(string, map[string]interface{}) error { return nil }
func (s *recordingAgentDataSaver) SaveGPUInfo(string, map[string]interface{}) error     { return nil }
func (s *recordingAgentDataSaver) SaveAgentLogs(string, []interface{}) error            { return nil }

func TestAgentReportIngestorRoutesMetricsThroughOneInterface(t *testing.T) {
	saver := &recordingAgentDataSaver{}
	ingestor := NewAgentReportIngestor(saver, nil)
	payload := map[string]interface{}{"cpu_usage": 42.0}

	if err := ingestor.Ingest("server-1", "metrics", payload); err != nil {
		t.Fatalf("ingest metrics: %v", err)
	}
	if saver.metrics["cpu_usage"] != 42.0 {
		t.Fatalf("metrics payload was not routed: %#v", saver.metrics)
	}
}

func TestAgentReportIngestorRejectsInvalidPayloadAndUnknownType(t *testing.T) {
	ingestor := NewAgentReportIngestor(&recordingAgentDataSaver{}, nil)

	if err := ingestor.Ingest("server-1", "metrics", []interface{}{}); err == nil {
		t.Fatal("metrics must reject a non-object payload")
	}
	if err := ingestor.Ingest("server-1", "unknown", map[string]interface{}{}); err == nil {
		t.Fatal("unknown report type must be rejected")
	}
}

func TestAgentReportIngestorRoutesServiceCheckResult(t *testing.T) {
	var got map[string]interface{}
	ingestor := NewAgentReportIngestor(&recordingAgentDataSaver{}, func(payload map[string]interface{}, serverID string) {
		if serverID != "server-1" {
			t.Errorf("server ID = %q", serverID)
		}
		got = payload
	})

	payload := map[string]interface{}{"status": "down", "error": errors.New("ignored")}
	if err := ingestor.Ingest("server-1", "service_check_result", payload); err != nil {
		t.Fatalf("ingest service check result: %v", err)
	}
	if got["status"] != "down" {
		t.Fatalf("service check payload was not routed: %#v", got)
	}
}
