package notification

import (
	"strings"
	"testing"
)

func TestDefaultAlertTemplatesRenderAllChannels(t *testing.T) {
	rendered, err := RenderAlertTemplates(DefaultAlertTemplates(), SampleAlertTemplateData())
	if err != nil {
		t.Fatalf("默认告警模板渲染失败: %v", err)
	}
	if !strings.Contains(rendered.EmailSubject, "示例服务器") {
		t.Fatalf("邮件主题未包含示例资源: %s", rendered.EmailSubject)
	}
	if !strings.Contains(rendered.EmailHTML, "CPU 使用率") {
		t.Fatalf("邮件 HTML 未包含摘要")
	}
	if !strings.Contains(rendered.EmailHTML, "background:#faad14") {
		t.Fatalf("邮件 HTML 未正确渲染状态颜色")
	}
	if !strings.Contains(rendered.WebhookText, "当前值: 86.50%") {
		t.Fatalf("Webhook 文本未包含展示字段: %s", rendered.WebhookText)
	}
}

func TestValidateAlertTemplatesRejectsUnknownVariable(t *testing.T) {
	templates := DefaultAlertTemplates()
	templates.EmailSubject = `{{ .Unknown }}`
	if err := ValidateAlertTemplates(templates); err == nil {
		t.Fatal("包含未知变量的模板应校验失败")
	}
}

func TestHTMLTemplateEscapesAlertData(t *testing.T) {
	data := SampleAlertTemplateData()
	data.Summary = `<script>alert("xss")</script>`
	rendered, err := RenderAlertTemplates(DefaultAlertTemplates(), data)
	if err != nil {
		t.Fatalf("模板渲染失败: %v", err)
	}
	if strings.Contains(rendered.EmailHTML, "<script>") {
		t.Fatal("告警数据必须经过 HTML 转义")
	}
}
