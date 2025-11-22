package services

import (
	"encoding/json"
	"fmt"
	"goravel/app/jobs"
	"goravel/app/utils/notification"
	"time"

	"github.com/goravel/framework/facades"
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

	// 从 system_settings 读取规则
	for _, metric := range []string{"cpu", "memory", "disk"} {
		var ruleJson string
		key := fmt.Sprintf("alert_rule_%s", metric)
		if err := facades.DB().Table("system_settings").Where("setting_key", key).Value("setting_value", &ruleJson); err == nil && ruleJson != "" {
			var rule Rule
			if err := json.Unmarshal([]byte(ruleJson), &rule); err == nil {
				switch metric {
				case "cpu":
					rules.CPU = rule
				case "memory":
					rules.Memory = rule
				case "disk":
					rules.Disk = rule
				}
			}
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
			// 设置冷却期（5分钟）
			facades.Cache().Put(cooldownKey, true, 5*time.Minute)
		} else {
			return nil
		}
	}

	// 更新状态
	facades.Cache().Put(cacheKey, string(newState), 24*time.Hour)

	// 如果恢复到正常状态，发送恢复通知
	if newState == AlertStateNormal && currentState != AlertStateNormal {
		s.sendNotification(serverID, metricName, value, newState, severity, true)
		return nil
	}

	// 如果进入告警状态，发送告警通知
	if newState != AlertStateNormal {
		s.sendNotification(serverID, metricName, value, newState, severity, false)
	}

	return nil
}

// sendNotification 发送通知
func (s *AlertService) sendNotification(serverID, metricName string, value float64, state AlertState, severity string, isRecovery bool) {
	// 获取服务器名称
	var serverName string
	var serverIP string
	var servers []map[string]interface{}
	err := facades.Orm().Query().Table("servers").
		Select("name", "ip").
		Where("id", serverID).
		Get(&servers)
	if err == nil && len(servers) > 0 {
		if name, ok := servers[0]["name"].(string); ok {
			serverName = name
		}
		if ip, ok := servers[0]["ip"].(string); ok {
			serverIP = ip
		}
	}
	if serverName == "" {
		serverName = serverID
	}
	if serverIP == "" {
		serverIP = "未知"
	}

	// 构建消息
	metricLabel := map[string]string{
		"cpu":    "CPU使用率",
		"memory": "内存使用率",
		"disk":   "磁盘使用率",
	}[metricName]

	var title, message string
	if isRecovery {
		title = fmt.Sprintf("[恢复] %s - %s", serverName, metricLabel)
		message = fmt.Sprintf("服务器 %s (%s) 的 %s 已恢复正常，当前值: %.2f%%", serverName, serverIP, metricLabel, value)
	} else {
		title = fmt.Sprintf("[%s] %s - %s", severity, serverName, metricLabel)
		message = fmt.Sprintf("服务器 %s (%s) 的 %s 达到 %s 阈值，当前值: %.2f%%", serverName, serverIP, metricLabel, severity, value)
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
			Content: title + "\n" + message,
		}).Dispatch(); err != nil {
			facades.Log().Errorf("分发Webhook发送任务失败: %v", err)
		}
	}
}

// getNotificationConfigs 获取通知配置
func (s *AlertService) getNotificationConfigs() (*notification.EmailConfig, *notification.WebhookConfig, error) {
	emailConfig := &notification.EmailConfig{Enabled: false}
	webhookConfig := &notification.WebhookConfig{Enabled: false}

	// 获取邮件配置
	var emailEnabled bool
	var emailConfigJson string
	facades.DB().Table("alert_notifications").Where("notification_type", "email").Value("enabled", &emailEnabled)
	facades.DB().Table("alert_notifications").Where("notification_type", "email").Value("config_json", &emailConfigJson)

	if emailEnabled && emailConfigJson != "" {
		if err := json.Unmarshal([]byte(emailConfigJson), &emailConfig); err == nil {
			emailConfig.Enabled = true
		}
	}

	// 获取Webhook配置
	var webhookEnabled bool
	var webhookConfigJson string
	facades.DB().Table("alert_notifications").Where("notification_type", "webhook").Value("enabled", &webhookEnabled)
	facades.DB().Table("alert_notifications").Where("notification_type", "webhook").Value("config_json", &webhookConfigJson)

	if webhookEnabled && webhookConfigJson != "" {
		if err := json.Unmarshal([]byte(webhookConfigJson), &webhookConfig); err == nil {
			webhookConfig.Enabled = true
		}
	}

	return emailConfig, webhookConfig, nil
}
