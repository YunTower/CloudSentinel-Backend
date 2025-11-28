package services

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"goravel/app/jobs"
	"goravel/app/repositories"
	"goravel/app/utils/notification"
	"html/template"
	"time"

	"github.com/goravel/framework/facades"
)

var (
	ResourceFiles embed.FS
)

// AlertService 告警服务
type AlertService struct{}

// NewAlertService 创建告警服务实例
func NewAlertService() *AlertService {
	return &AlertService{}
}

// Rule 告警规则
type Rule struct {
	Enabled  bool    `json:"enabled"`
	Warning  float64 `json:"warning"`
	Critical float64 `json:"critical"`
}

// AlertState 告警状态
type AlertState string

const (
	AlertStateNormal   AlertState = "normal"
	AlertStateWarning  AlertState = "warning"
	AlertStateCritical AlertState = "critical"
)

// CheckAndAlert 检查指标并触发告警
func (s *AlertService) CheckAndAlert(serverID string, metrics map[string]interface{}) error {
	// 获取告警规则
	rules, err := s.getRules()
	if err != nil {
		facades.Log().Warningf("获取告警规则失败: %v", err)
		return err
	}

	// 检查 CPU 告警
	if cpuUsage, ok := metrics["cpu_usage"].(float64); ok {
		if err := s.evaluateRule(serverID, "cpu", cpuUsage, rules.CPU); err != nil {
			facades.Log().Warningf("CPU告警检查失败: %v", err)
		}
	}

	// 检查内存告警
	if memoryUsage, ok := metrics["memory_usage"].(float64); ok {
		if err := s.evaluateRule(serverID, "memory", memoryUsage, rules.Memory); err != nil {
			facades.Log().Warningf("内存告警检查失败: %v", err)
		}
	}

	// 检查磁盘告警
	if diskUsage, ok := metrics["disk_usage"].(float64); ok {
		if err := s.evaluateRule(serverID, "disk", diskUsage, rules.Disk); err != nil {
			facades.Log().Warningf("磁盘告警检查失败: %v", err)
		}
	}

	return nil
}

// Rules 所有告警规则
type Rules struct {
	CPU    Rule `json:"cpu"`
	Memory Rule `json:"memory"`
	Disk   Rule `json:"disk"`
}

// getRules 获取所有告警规则
func (s *AlertService) getRules() (*Rules, error) {
	rules := &Rules{
		CPU:    Rule{Enabled: true, Warning: 80, Critical: 90},
		Memory: Rule{Enabled: true, Warning: 85, Critical: 95},
		Disk:   Rule{Enabled: true, Warning: 85, Critical: 95},
	}

	// 批量获取所有告警规则
	settingRepo := repositories.GetSystemSettingRepository()
	keys := []string{"alert_rule_cpu", "alert_rule_memory", "alert_rule_disk"}
	settings, err := settingRepo.GetByKeys(keys)

	if err != nil {
		return rules, nil // 使用默认规则
	}

	// 解析规则
	for key, setting := range settings {
		if setting == nil {
			continue
		}
		ruleJson := setting.GetValue()

		if ruleJson == "" {
			continue
		}

		var rule Rule
		if err := json.Unmarshal([]byte(ruleJson), &rule); err != nil {
			continue
		}

		// 根据key设置对应的规则
		switch key {
		case "alert_rule_cpu":
			rules.CPU = rule
		case "alert_rule_memory":
			rules.Memory = rule
		case "alert_rule_disk":
			rules.Disk = rule
		}
	}

	return rules, nil
}

// evaluateRule 评估单个规则
func (s *AlertService) evaluateRule(serverID, metricName string, value float64, rule Rule) error {
	if !rule.Enabled {
		return nil
	}

	// 获取当前告警状态
	cacheKey := fmt.Sprintf("alert_state:%s:%s", serverID, metricName)
	var currentState AlertState
	if cached := facades.Cache().Get(cacheKey); cached != nil {
		if stateStr, ok := cached.(string); ok {
			currentState = AlertState(stateStr)
		}
	}

	// 确定新状态
	var newState AlertState
	var severity string
	if value >= rule.Critical {
		newState = AlertStateCritical
		severity = "严重"
	} else if value >= rule.Warning {
		newState = AlertStateWarning
		severity = "警告"
	} else {
		newState = AlertStateNormal
	}

	// 如果状态没有变化，且不是从告警状态恢复到正常，则不发送通知
	if newState == currentState {
		// 如果当前是告警状态，检查是否需要重新发送（冷却期）
		if newState != AlertStateNormal {
			cooldownKey := fmt.Sprintf("alert_cooldown:%s:%s", serverID, metricName)
			if cooldown := facades.Cache().Get(cooldownKey); cooldown != nil {
				// 还在冷却期内，不发送
				return nil
			}
			// 设置冷却期（2分钟）
			err := facades.Cache().Put(cooldownKey, true, 2*time.Minute)
			if err != nil {
				return err
			}
		} else {
			return nil
		}
	}

	// 更新状态
	err := facades.Cache().Put(cacheKey, string(newState), 24*time.Hour)
	if err != nil {
		return err
	}

	// 如果恢复到正常状态，发送恢复通知
	if newState == AlertStateNormal && currentState != AlertStateNormal {
		s.sendNotification(serverID, metricName, value, newState, severity, true, rule)
		return nil
	}

	// 如果进入告警状态，发送告警通知
	if newState != AlertStateNormal {
		s.sendNotification(serverID, metricName, value, newState, severity, false, rule)
	}

	return nil
}

// sendNotification 发送通知
func (s *AlertService) sendNotification(serverID, metricName string, value float64, state AlertState, severity string, isRecovery bool, rule Rule) {
	// 获取服务器名称
	serverRepo := repositories.GetServerRepository()
	server, err := serverRepo.GetByID(serverID)
	serverName := serverID
	serverIP := "未知"
	if err == nil && server != nil {
		serverName = server.Name
		serverIP = server.IP
	}

	// 构建消息
	metricLabel := map[string]string{
		"cpu":    "CPU使用率",
		"memory": "内存使用率",
		"disk":   "磁盘使用率",
	}[metricName]

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var title, message, webhookMessage string
	var threshold float64
	if severity == "警告" {
		threshold = rule.Warning
	} else {
		threshold = rule.Critical
	}

	if isRecovery {
		title = fmt.Sprintf("[恢复] %s - %s", serverName, metricLabel)
		webhookMessage = fmt.Sprintf("✅ 告警恢复\n\n服务器: %s (%s)\n指标: %s\n当前值: %.2f%%\n恢复时间: %s",
			serverName, serverIP, metricLabel, value, timestamp)
	} else {
		title = fmt.Sprintf("[%s] %s - %s", severity, serverName, metricLabel)
		webhookMessage = fmt.Sprintf("🚨 发生告警 (%s)\n\n服务器: %s (%s)\n指标: %s\n当前值: %.2f%%\n阈值: %.2f%%\n触发时间: %s",
			severity, serverName, serverIP, metricLabel, value, threshold, timestamp)
	}

	color := "#ff4d4f" // 红色
	if severity == "警告" {
		color = "#faad14" // 橙色
	}
	if isRecovery {
		color = "#52c41a" // 绿色
	}

	statusText := severity
	if isRecovery {
		statusText = "恢复正常"
	}

	templateData := map[string]interface{}{
		"Title":        title,
		"Timestamp":    timestamp,
		"ServerName":   serverName,
		"ServerIP":     serverIP,
		"MetricLabel":  metricLabel,
		"StatusText":   statusText,
		"Color":        color,
		"CurrentValue": value,
		"Threshold":    threshold,
	}

	var tmpl *template.Template
	var templateErr error

	templateContent, err := ResourceFiles.ReadFile("resources/views/emails/alert.tmpl")
	if err == nil {
		tmpl, templateErr = template.New("emails/alert.tmpl").Parse(string(templateContent))
	} else {
		templateErr = err
	}
	if templateErr != nil {
		facades.Log().Warningf("解析邮件模板失败: %v", templateErr)
		if isRecovery {
			message = fmt.Sprintf("告警恢复通知\n\n服务器: %s (%s)\n指标: %s\n当前值: %.2f%%\n恢复时间: %s\n\n此邮件由云哨监控系统自动发送，请勿回复。",
				serverName, serverIP, metricLabel, value, timestamp)
		} else {
			message = fmt.Sprintf("告警通知 (%s)\n\n服务器: %s (%s)\n指标: %s\n当前状态: %s\n当前值: %.2f%%\n触发阈值: %.2f%%\n触发时间: %s\n\n此邮件由云哨监控系统自动发送，请勿回复。",
				severity, serverName, serverIP, metricLabel, statusText, value, threshold, timestamp)
		}
	} else {
		var buf bytes.Buffer
		templateName := "emails/alert.tmpl"
		if execErr := tmpl.ExecuteTemplate(&buf, templateName, templateData); execErr != nil {
			facades.Log().Errorf("渲染邮件模板失败: %v", execErr)
			if isRecovery {
				message = fmt.Sprintf("告警恢复通知\n\n服务器: %s (%s)\n指标: %s\n当前值: %.2f%%\n恢复时间: %s\n\n此邮件由云哨监控系统自动发送，请勿回复。",
					serverName, serverIP, metricLabel, value, timestamp)
			} else {
				message = fmt.Sprintf("告警通知 (%s)\n\n服务器: %s (%s)\n指标: %s\n当前状态: %s\n当前值: %.2f%%\n触发阈值: %.2f%%\n触发时间: %s\n\n此邮件由云哨监控系统自动发送，请勿回复。",
					severity, serverName, serverIP, metricLabel, statusText, value, threshold, timestamp)
			}
		} else {
			message = buf.String()
		}
	}

	// 获取通知配置并发送
	emailConfig, webhookConfig, err := s.getNotificationConfigs()
	if err != nil {
		facades.Log().Warningf("获取通知配置失败: %v", err)
		return
	}

	// 发送邮件
	if emailConfig.Enabled {
		configJson, _ := json.Marshal(emailConfig)
		if err := facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "email",
			Config:  string(configJson),
			Subject: title,
			Content: message,
		}).Dispatch(); err != nil {
			facades.Log().Errorf("分发邮件发送任务失败: %v", err)
		}
	}

	// 发送Webhook
	if webhookConfig.Enabled {
		configJson, _ := json.Marshal(webhookConfig)
		if err := facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "webhook",
			Config:  string(configJson),
			Subject: title,
			Content: webhookMessage,
		}).Dispatch(); err != nil {
			facades.Log().Errorf("分发Webhook发送任务失败: %v", err)
		}
	}
}

// getNotificationConfigs 获取通知配置
func (s *AlertService) getNotificationConfigs() (*notification.EmailConfig, *notification.WebhookConfig, error) {
	emailConfig := &notification.EmailConfig{Enabled: false}
	webhookConfig := &notification.WebhookConfig{Enabled: false}

	notificationRepo := repositories.GetAlertNotificationRepository()
	notifications, err := notificationRepo.GetAll()

	if err != nil {
		return emailConfig, webhookConfig, err
	}

	// 解析配置
	for _, notif := range notifications {
		if !notif.Enabled || notif.ConfigJson == "" {
			continue
		}

		switch notif.NotificationType {
		case "email":
			if err := json.Unmarshal([]byte(notif.ConfigJson), &emailConfig); err == nil {
				emailConfig.Enabled = true
			}
		case "webhook":
			if err := json.Unmarshal([]byte(notif.ConfigJson), &webhookConfig); err == nil {
				webhookConfig.Enabled = true
			}
		}
	}

	return emailConfig, webhookConfig, nil
}
