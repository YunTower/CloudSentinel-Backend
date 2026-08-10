package controllers

import (
	"testing"

	"goravel/app/models"
	"goravel/app/monitorprobe"
)

func TestPrepareProtocolMonitor设置Minecraft默认端口(t *testing.T) {
	monitor := &models.ServiceMonitor{Type: monitorprobe.TypeMinecraftJava, Target: " mc.example.com "}
	if err := prepareProtocolMonitor(monitor, ""); err != nil {
		t.Fatal(err)
	}
	if monitor.Target != "mc.example.com" || monitor.Port != 25565 {
		t.Fatalf("Minecraft 配置未规范化: %#v", monitor)
	}
}

func TestValidateProtocolMonitorUpdate保留现有AI密钥(t *testing.T) {
	existing := &models.ServiceMonitor{
		Type: monitorprobe.TypeAIModel, Target: "https://api.example.com/v1/responses",
		AIAPIFormat: monitorprobe.AIFormatResponses, AIModel: "gpt-5-mini", AIAPIKeyEncrypted: "enc:value",
	}
	if err := validateProtocolMonitorUpdate(existing, "", "", nil, nil, nil, nil); err != nil {
		t.Fatalf("留空更新应保留现有密钥: %v", err)
	}
}

func TestValidateProtocolMonitorUpdate拒绝清空AI密钥(t *testing.T) {
	existing := &models.ServiceMonitor{
		Type: monitorprobe.TypeAIModel, Target: "https://api.example.com/v1/responses",
		AIAPIFormat: monitorprobe.AIFormatResponses, AIModel: "gpt-5-mini", AIAPIKeyEncrypted: "enc:value",
	}
	clear := true
	if err := validateProtocolMonitorUpdate(existing, "", "", nil, nil, nil, &clear); err == nil {
		t.Fatal("清空 AI 密钥后仍允许保存")
	}
}

func TestValidateFullHTTPURL拒绝非完整地址(t *testing.T) {
	for _, value := range []string{"api.example.com/v1/messages", "/v1/messages", "ftp://api.example.com/model"} {
		if err := validateFullHTTPURL(value); err == nil {
			t.Fatalf("应拒绝地址 %q", value)
		}
	}
}
