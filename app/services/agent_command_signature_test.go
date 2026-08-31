package services

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCanonicalAgentCommandJSONNormalizationStability 锚定命令签名规范化的
// 字节级行为。Agent 端（agent/internal/reporter/command_signature_test.go）
// 有一条镜像测试；两端必须对同一数据产出相同字节，任一端更换 JSON 库或
// 关闭 HTML 转义都会导致全线验签失败——改动前先跑两端测试。
func TestCanonicalAgentCommandJSONNormalizationStability(t *testing.T) {
	original := map[string]interface{}{
		"monitor_id":   3,
		"port":         443,
		"timeout":      10,
		"target":       "https://例え.com/a?b=<script>&c=\"d\"",
		"http_headers": `{"X-Token":"a<b>&c"}`,
		"nested":       map[string]interface{}{"z": 1, "a": "值"},
		"list":         []interface{}{"x", 2},
	}

	backendBytes, err := canonicalAgentCommandJSON(original)
	if err != nil {
		t.Fatal(err)
	}

	// Agent 路径：JSON roundtrip（模拟消息经网络解码为 float64 map）后重新序列化
	var roundtripped map[string]interface{}
	if err := json.Unmarshal(backendBytes, &roundtripped); err != nil {
		t.Fatal(err)
	}
	agentBytes, err := json.Marshal(roundtripped)
	if err != nil {
		t.Fatal(err)
	}

	if string(backendBytes) != string(agentBytes) {
		t.Fatalf("canonical JSON mismatch:\nbackend: %s\nagent:   %s", backendBytes, agentBytes)
	}

	if !strings.Contains(string(backendBytes), `\u003c`) {
		t.Fatal("HTML escaping behavior changed (< not escaped); agent verification will break")
	}
}
