package notification

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
	"time"

	"goravel/app/facades"
	"goravel/app/repositories"
)

const alertTemplatesSettingKey = "alert_notification_templates_v1"

const (
	maxEmailSubjectTemplateBytes = 500
	maxEmailHTMLTemplateBytes    = 64 * 1024
	maxWebhookTemplateBytes      = 8 * 1024
)

// AlertTemplates 是管理员可编辑的统一告警模板。版本字段用于后续平滑扩展模板契约。
type AlertTemplates struct {
	Version      int    `json:"version"`
	EmailSubject string `json:"emailSubject"`
	EmailHTML    string `json:"emailHtml"`
	WebhookText  string `json:"webhookText"`
}

// AlertTemplateField 是模板中可遍历的展示字段。
type AlertTemplateField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// AlertTemplateData 是所有告警类型共享的稳定模板变量契约。
type AlertTemplateData struct {
	Event           string               `json:"event"`
	Status          string               `json:"status"`
	Severity        string               `json:"severity"`
	Title           string               `json:"title"`
	Summary         string               `json:"summary"`
	ResourceName    string               `json:"resourceName"`
	ResourceType    string               `json:"resourceType"`
	ResourceAddress string               `json:"resourceAddress"`
	OccurredAt      string               `json:"occurredAt"`
	Color           string               `json:"color"`
	Fields          []AlertTemplateField `json:"fields"`
}

// RenderedAlert 是渲染完成、可直接交给通知渠道的确定性内容。
type RenderedAlert struct {
	EmailSubject string `json:"emailSubject"`
	EmailHTML    string `json:"emailHtml"`
	WebhookText  string `json:"webhookText"`
}

func DefaultAlertTemplates() AlertTemplates {
	return AlertTemplates{
		Version:      1,
		EmailSubject: `{{ .Title }}`,
		EmailHTML: `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;background:#f5f7fa;font-family:Arial,'Microsoft YaHei',sans-serif;color:#1f2937">
  <div style="max-width:680px;margin:0 auto;padding:24px">
    <div style="background:#fff;border-radius:10px;overflow:hidden;border:1px solid #e5e7eb">
      <div style="padding:18px 24px;background:{{ .Color }};color:#fff"><h2 style="margin:0;font-size:20px">{{ .Title }}</h2></div>
      <div style="padding:24px">
        <p style="margin:0 0 18px;line-height:1.7">{{ .Summary }}</p>
        <table style="width:100%;border-collapse:collapse;font-size:14px">
          <tr><td style="padding:8px 0;color:#6b7280">资源</td><td style="padding:8px 0;text-align:right">{{ .ResourceName }}</td></tr>
          {{ if .ResourceAddress }}<tr><td style="padding:8px 0;color:#6b7280">地址</td><td style="padding:8px 0;text-align:right">{{ .ResourceAddress }}</td></tr>{{ end }}
          {{ range .Fields }}<tr><td style="padding:8px 0;color:#6b7280">{{ .Label }}</td><td style="padding:8px 0;text-align:right">{{ .Value }}</td></tr>{{ end }}
          <tr><td style="padding:8px 0;color:#6b7280">时间</td><td style="padding:8px 0;text-align:right">{{ .OccurredAt }}</td></tr>
        </table>
      </div>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px">此邮件由 CloudSentinel 自动发送，请勿回复。</p>
  </div>
</body>
</html>`,
		WebhookText: `{{ if eq .Status "recovery" }}✅{{ else }}🚨{{ end }} {{ .Title }}
{{ .Summary }}
资源: {{ .ResourceName }}{{ if .ResourceAddress }} ({{ .ResourceAddress }}){{ end }}
{{ range .Fields }}{{ .Label }}: {{ .Value }}
{{ end }}时间: {{ .OccurredAt }}`,
	}
}

func SampleAlertTemplateData() AlertTemplateData {
	return AlertTemplateData{
		Event:           "metric.cpu",
		Status:          "alert",
		Severity:        "warning",
		Title:           "[警告] 示例服务器 - CPU使用率",
		Summary:         "CPU 使用率已超过配置的告警阈值。",
		ResourceName:    "示例服务器",
		ResourceType:    "server",
		ResourceAddress: "192.0.2.10",
		OccurredAt:      time.Now().Format("2006-01-02 15:04:05"),
		Color:           "#faad14",
		Fields: []AlertTemplateField{
			{Label: "当前值", Value: "86.50%"},
			{Label: "触发阈值", Value: "80.00%"},
		},
	}
}

func ValidateAlertTemplates(templates AlertTemplates) error {
	if templates.Version == 0 {
		templates.Version = 1
	}
	if templates.Version != 1 {
		return fmt.Errorf("不支持的模板版本")
	}
	if strings.TrimSpace(templates.EmailSubject) == "" || len(templates.EmailSubject) > maxEmailSubjectTemplateBytes {
		return fmt.Errorf("邮件主题模板不能为空且不能超过 %d 字节", maxEmailSubjectTemplateBytes)
	}
	if strings.TrimSpace(templates.EmailHTML) == "" || len(templates.EmailHTML) > maxEmailHTMLTemplateBytes {
		return fmt.Errorf("邮件 HTML 模板不能为空且不能超过 %d 字节", maxEmailHTMLTemplateBytes)
	}
	if strings.TrimSpace(templates.WebhookText) == "" || len(templates.WebhookText) > maxWebhookTemplateBytes {
		return fmt.Errorf("Webhook 模板不能为空且不能超过 %d 字节", maxWebhookTemplateBytes)
	}
	_, err := RenderAlertTemplates(templates, SampleAlertTemplateData())
	return err
}

func RenderAlertTemplates(templates AlertTemplates, data AlertTemplateData) (RenderedAlert, error) {
	if templates.Version == 0 {
		templates.Version = 1
	}
	if data.Color == "" {
		data.Color = "#d03050"
	}

	subjectTemplate, err := texttemplate.New("emailSubject").Option("missingkey=error").Parse(templates.EmailSubject)
	if err != nil {
		return RenderedAlert{}, fmt.Errorf("邮件主题模板语法错误: %w", err)
	}
	htmlTemplate, err := htmltemplate.New("emailHtml").Option("missingkey=error").Parse(templates.EmailHTML)
	if err != nil {
		return RenderedAlert{}, fmt.Errorf("邮件 HTML 模板语法错误: %w", err)
	}
	webhookTemplate, err := texttemplate.New("webhookText").Option("missingkey=error").Parse(templates.WebhookText)
	if err != nil {
		return RenderedAlert{}, fmt.Errorf("Webhook 模板语法错误: %w", err)
	}

	var subject, html, webhook bytes.Buffer
	if err := subjectTemplate.Execute(&subject, data); err != nil {
		return RenderedAlert{}, fmt.Errorf("邮件主题模板渲染失败: %w", err)
	}
	if err := htmlTemplate.Execute(&html, data); err != nil {
		return RenderedAlert{}, fmt.Errorf("邮件 HTML 模板渲染失败: %w", err)
	}
	if err := webhookTemplate.Execute(&webhook, data); err != nil {
		return RenderedAlert{}, fmt.Errorf("Webhook 模板渲染失败: %w", err)
	}

	return RenderedAlert{
		EmailSubject: strings.TrimSpace(subject.String()),
		EmailHTML:    html.String(),
		WebhookText:  strings.TrimSpace(webhook.String()),
	}, nil
}

func LoadAlertTemplates() AlertTemplates {
	defaults := DefaultAlertTemplates()
	var templates AlertTemplates
	if err := repositories.GetSystemSettingRepository().GetJSONWithDefault(alertTemplatesSettingKey, &templates, defaults); err != nil {
		return defaults
	}
	if err := ValidateAlertTemplates(templates); err != nil {
		facades.Log().Warningf("告警模板配置无效，已回退内置模板: %v", err)
		return defaults
	}
	return templates
}

func SaveAlertTemplates(templates AlertTemplates) error {
	if templates.Version == 0 {
		templates.Version = 1
	}
	if err := ValidateAlertTemplates(templates); err != nil {
		return err
	}
	return repositories.GetSystemSettingRepository().SetJSON(alertTemplatesSettingKey, templates)
}

func RenderConfiguredAlert(data AlertTemplateData) RenderedAlert {
	templates := LoadAlertTemplates()
	rendered, err := RenderAlertTemplates(templates, data)
	if err == nil {
		return rendered
	}
	facades.Log().Warningf("渲染告警模板失败，已回退内置模板: %v", err)
	rendered, _ = RenderAlertTemplates(DefaultAlertTemplates(), data)
	return rendered
}
