package services

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"goravel/app/jobs"
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/utils"
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
	// 获取告警规则（使用服务器特定规则）
	serverIDPtr := &serverID
	rules, err := s.GetServerRules(serverIDPtr)
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

// getRules 获取所有告警规则（兼容旧接口，使用全局规则）
func (s *AlertService) getRules() (*Rules, error) {
	return s.GetServerRules(nil)
}

// GetServerRules 获取指定服务器的告警规则
func (s *AlertService) GetServerRules(serverID *string) (*Rules, error) {
	// 默认规则（禁用状态）
	defaultRules := &Rules{
		CPU:    Rule{Enabled: false, Warning: 80, Critical: 90},
		Memory: Rule{Enabled: false, Warning: 85, Critical: 95},
		Disk:   Rule{Enabled: false, Warning: 85, Critical: 95},
	}

	// 如果没有指定服务器ID，返回禁用状态的默认规则
	if serverID == nil {
		return defaultRules, nil
	}

	ruleRepo := repositories.GetServerAlertRuleRepository()
	ruleTypes := []string{"cpu", "memory", "disk"}

	// 获取服务器特定规则
	serverRules := make(map[string]*Rule)
	serverRuleList, err := ruleRepo.GetByServerID(*serverID)
	if err == nil {
		for _, ruleRecord := range serverRuleList {
			var rule Rule
			if err := json.Unmarshal([]byte(ruleRecord.Config), &rule); err == nil {
				serverRules[ruleRecord.RuleType] = &rule
			}
		}
	}

	// 只使用服务器特定规则，没有则使用禁用状态的默认规则
	result := &Rules{}
	for _, ruleType := range ruleTypes {
		var rule *Rule
		// 只使用服务器特定规则
		if r, ok := serverRules[ruleType]; ok {
			rule = r
		} else {
			// 使用禁用状态的默认规则
			switch ruleType {
			case "cpu":
				rule = &defaultRules.CPU
			case "memory":
				rule = &defaultRules.Memory
			case "disk":
				rule = &defaultRules.Disk
			}
		}

		// 设置结果
		switch ruleType {
		case "cpu":
			result.CPU = *rule
		case "memory":
			result.Memory = *rule
		case "disk":
			result.Disk = *rule
		}
	}

	return result, nil
}

// SaveServerRules 保存服务器告警规则（serverID 不能为 nil）
func (s *AlertService) SaveServerRules(serverID *string, rules map[string]Rule) error {
	if serverID == nil {
		return fmt.Errorf("serverID cannot be nil")
	}

	ruleRepo := repositories.GetServerAlertRuleRepository()

	for ruleType, rule := range rules {
		ruleJson, err := json.Marshal(rule)
		if err != nil {
			facades.Log().Warningf("序列化告警规则失败 %s: %v", ruleType, err)
			continue
		}

		ruleRecord := &models.ServerAlertRule{
			ServerID: serverID,
			RuleType: ruleType,
			Config:   string(ruleJson),
		}

		if err := ruleRepo.CreateOrUpdate(ruleRecord); err != nil {
			facades.Log().Warningf("保存告警规则失败 %s: %v", ruleType, err)
			return err
		}
	}

	return nil
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

	// 获取通知配置并发送（优先使用服务器特定配置）
	emailConfig, webhookConfig, err := s.getNotificationConfigs(serverID)
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
func (s *AlertService) getNotificationConfigs(serverID string) (*notification.EmailConfig, *notification.WebhookConfig, error) {
	emailConfig := &notification.EmailConfig{Enabled: false}
	webhookConfig := &notification.WebhookConfig{Enabled: false}

	// 获取全局通知配置
	notificationRepo := repositories.GetAlertNotificationRepository()
	notifications, err := notificationRepo.GetAll()
	if err != nil {
		return emailConfig, webhookConfig, err
	}

	// 解析全局配置
	for _, notif := range notifications {
		if !notif.Enabled || notif.ConfigJson == "" {
			continue
		}

		switch notif.NotificationType {
		case "email":
			if err := json.Unmarshal([]byte(notif.ConfigJson), &emailConfig); err == nil {
			}
		case "webhook":
			if err := json.Unmarshal([]byte(notif.ConfigJson), &webhookConfig); err == nil {
			}
		}
	}

	// 获取服务器特定的通知渠道配置
	channelRepo := repositories.GetServerNotificationChannelRepository()
	serverChannels, err := channelRepo.GetByServerID(serverID)
	if err == nil && len(serverChannels) > 0 {
		// 只使用服务器配置的启用状态
		for _, channel := range serverChannels {
			if channel.NotificationType == "email" {
				emailConfig.Enabled = channel.Enabled
			} else if channel.NotificationType == "webhook" {
				webhookConfig.Enabled = channel.Enabled
			}
		}
	}

	return emailConfig, webhookConfig, nil
}

// GetServerNotificationChannels 获取服务器启用的通知渠道
func (s *AlertService) GetServerNotificationChannels(serverID string) (map[string]bool, error) {
	channelRepo := repositories.GetServerNotificationChannelRepository()
	channels, err := channelRepo.GetByServerID(serverID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, channel := range channels {
		result[channel.NotificationType] = channel.Enabled
	}

	return result, nil
}

// SaveServerNotificationChannels 保存服务器通知渠道配置
func (s *AlertService) SaveServerNotificationChannels(serverID string, channels map[string]bool) error {
	channelRepo := repositories.GetServerNotificationChannelRepository()

	for notificationType, enabled := range channels {
		// 验证 notificationType 不为空
		if notificationType == "" {
			continue
		}

		channel := &models.ServerNotificationChannel{
			ServerID:         serverID,
			NotificationType: notificationType,
			Enabled:          enabled,
		}

		if err := channelRepo.CreateOrUpdate(channel); err != nil {
			facades.Log().Warningf("保存服务器通知渠道配置失败 %s: %v", notificationType, err)
			return err
		}
	}

	return nil
}

// CheckBandwidth 检查带宽峰值告警
func (s *AlertService) CheckBandwidth(serverID string, currentMbps float64) error {
	serverIDPtr := &serverID
	ruleRepo := repositories.GetServerAlertRuleRepository()

	// 获取服务器特定规则
	rule, err := ruleRepo.GetByServerIDAndType(serverIDPtr, "bandwidth")
	if err != nil {
		// 没有配置规则，不检查
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(rule.Config), &config); err != nil {
		return nil
	}

	enabled, _ := config["enabled"].(bool)
	if !enabled {
		return nil
	}

	threshold, ok := config["threshold"].(float64)
	if !ok {
		return nil
	}

	if currentMbps >= threshold {
		// 触发告警
		serverRepo := repositories.GetServerRepository()
		server, err := serverRepo.GetByID(serverID)
		serverName := serverID
		serverIP := "未知"
		if err == nil && server != nil {
			serverName = server.Name
			serverIP = server.IP
		}

		title := fmt.Sprintf("[告警] %s - 带宽峰值", serverName)
		webhookMessage := fmt.Sprintf("🚨 带宽峰值告警\n\n服务器: %s (%s)\n当前带宽: %.2f Mbps\n阈值: %.2f Mbps\n触发时间: %s",
			serverName, serverIP, currentMbps, threshold, time.Now().Format("2006-01-02 15:04:05"))

		// 检查冷却期
		cacheKey := fmt.Sprintf("alert_cooldown:%s:bandwidth", serverID)
		if cooldown := facades.Cache().Get(cacheKey); cooldown != nil {
			return nil
		}
		facades.Cache().Put(cacheKey, true, 2*time.Minute)

		// 发送通知
		emailConfig, webhookConfig, _ := s.getNotificationConfigs(serverID)
		if emailConfig.Enabled {
			configJson, _ := json.Marshal(emailConfig)
			_ = facades.Queue().Job(&jobs.SendAlertJob{
				Channel: "email",
				Config:  string(configJson),
				Subject: title,
				Content: webhookMessage,
			}).Dispatch()
		}
		if webhookConfig.Enabled {
			configJson, _ := json.Marshal(webhookConfig)
			_ = facades.Queue().Job(&jobs.SendAlertJob{
				Channel: "webhook",
				Config:  string(configJson),
				Subject: title,
				Content: webhookMessage,
			}).Dispatch()
		}
	}

	return nil
}

// CheckTraffic 检查流量耗尽告警
func (s *AlertService) CheckTraffic(serverID string, usedBytes int64, limitBytes int64) error {
	if limitBytes <= 0 {
		// 无限制，不检查
		return nil
	}

	serverIDPtr := &serverID
	ruleRepo := repositories.GetServerAlertRuleRepository()

	// 获取服务器特定规则
	rule, err := ruleRepo.GetByServerIDAndType(serverIDPtr, "traffic")
	if err != nil {
		// 没有配置规则，不检查
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(rule.Config), &config); err != nil {
		return nil
	}

	enabled, _ := config["enabled"].(bool)
	if !enabled {
		return nil
	}

	thresholdPercent, ok := config["threshold_percent"].(float64)
	if !ok {
		return nil
	}

	usedPercent := float64(usedBytes) / float64(limitBytes) * 100
	if usedPercent >= thresholdPercent {
		// 触发告警
		serverRepo := repositories.GetServerRepository()
		server, err := serverRepo.GetByID(serverID)
		serverName := serverID
		serverIP := "未知"
		if err == nil && server != nil {
			serverName = server.Name
			serverIP = server.IP
		}

		usedGB := float64(usedBytes) / (1024 * 1024 * 1024)
		limitGB := float64(limitBytes) / (1024 * 1024 * 1024)

		title := fmt.Sprintf("[告警] %s - 流量耗尽", serverName)
		webhookMessage := fmt.Sprintf("🚨 流量耗尽告警\n\n服务器: %s (%s)\n已用流量: %.2f GB / %.2f GB (%.2f%%)\n阈值: %.2f%%\n触发时间: %s",
			serverName, serverIP, usedGB, limitGB, usedPercent, thresholdPercent, time.Now().Format("2006-01-02 15:04:05"))

		// 检查冷却期
		cacheKey := fmt.Sprintf("alert_cooldown:%s:traffic", serverID)
		if cooldown := facades.Cache().Get(cacheKey); cooldown != nil {
			return nil
		}
		facades.Cache().Put(cacheKey, true, 2*time.Minute)

		// 发送通知
		emailConfig, webhookConfig, _ := s.getNotificationConfigs(serverID)
		if emailConfig.Enabled {
			configJson, _ := json.Marshal(emailConfig)
			_ = facades.Queue().Job(&jobs.SendAlertJob{
				Channel: "email",
				Config:  string(configJson),
				Subject: title,
				Content: webhookMessage,
			}).Dispatch()
		}
		if webhookConfig.Enabled {
			configJson, _ := json.Marshal(webhookConfig)
			_ = facades.Queue().Job(&jobs.SendAlertJob{
				Channel: "webhook",
				Config:  string(configJson),
				Subject: title,
				Content: webhookMessage,
			}).Dispatch()
		}
	}

	return nil
}

// CheckExpiration 检查服务器到期告警
func (s *AlertService) CheckExpiration(serverID string) error {
	serverRepo := repositories.GetServerRepository()
	server, err := serverRepo.GetByID(serverID)
	if err != nil || server == nil {
		return nil
	}

	if server.ExpireTime == nil {
		// 无到期时间，不检查
		return nil
	}

	serverIDPtr := &serverID
	ruleRepo := repositories.GetServerAlertRuleRepository()

	// 获取服务器特定规则
	rule, err := ruleRepo.GetByServerIDAndType(serverIDPtr, "expiration")
	if err != nil {
		// 没有配置规则，不检查
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(rule.Config), &config); err != nil {
		return nil
	}

	enabled, _ := config["enabled"].(bool)
	if !enabled {
		return nil
	}

	alertDays, ok := config["alert_days"].(float64)
	if !ok {
		return nil
	}

	now := time.Now()
	expireTime := *server.ExpireTime
	daysUntilExpire := expireTime.Sub(now).Hours() / 24

	if daysUntilExpire <= alertDays && daysUntilExpire >= 0 {
		// 触发告警
		title := fmt.Sprintf("[告警] %s - 即将到期", server.Name)
		webhookMessage := fmt.Sprintf("🚨 服务器到期提醒\n\n服务器: %s (%s)\n到期时间: %s\n剩余天数: %.0f 天\n触发时间: %s",
			server.Name, server.IP, expireTime.Format("2006-01-02 15:04:05"), daysUntilExpire, now.Format("2006-01-02 15:04:05"))

		// 检查冷却期（每天只发送一次）
		cacheKey := fmt.Sprintf("alert_cooldown:%s:expiration", serverID)
		if cooldown := facades.Cache().Get(cacheKey); cooldown != nil {
			return nil
		}
		facades.Cache().Put(cacheKey, true, 24*time.Hour)

		// 发送通知
		emailConfig, webhookConfig, _ := s.getNotificationConfigs(serverID)
		if emailConfig.Enabled {
			configJson, _ := json.Marshal(emailConfig)
			_ = facades.Queue().Job(&jobs.SendAlertJob{
				Channel: "email",
				Config:  string(configJson),
				Subject: title,
				Content: webhookMessage,
			}).Dispatch()
		}
		if webhookConfig.Enabled {
			configJson, _ := json.Marshal(webhookConfig)
			_ = facades.Queue().Job(&jobs.SendAlertJob{
				Channel: "webhook",
				Config:  string(configJson),
				Subject: title,
				Content: webhookMessage,
			}).Dispatch()
		}
	}

	return nil
}

// NotifyServerOffline 发送服务器离线告警
func (s *AlertService) NotifyServerOffline(serverID string) {
	if !utils.GetSettingBool("alert_server_offline_enabled", false) {
		return
	}
	cacheKey := fmt.Sprintf("alert_cooldown:%s:server_offline", serverID)
	if facades.Cache().Get(cacheKey) != nil {
		return
	}
	_ = facades.Cache().Put(cacheKey, true, 5*time.Minute)

	emailConfig, webhookConfig, err := s.getNotificationConfigs(serverID)
	if err != nil || (!emailConfig.Enabled && !webhookConfig.Enabled) {
		return
	}

	serverRepo := repositories.GetServerRepository()
	server, _ := serverRepo.GetByID(serverID)
	serverName, serverIP := serverID, "未知"
	if server != nil {
		serverName, serverIP = server.Name, server.IP
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[告警] %s - 服务器离线", serverName)
	content := fmt.Sprintf("🚨 服务器离线告警\n\n服务器: %s (%s)\n离线时间: %s\n\n请检查该服务器或 Agent 状态。",
		serverName, serverIP, timestamp)

	if emailConfig.Enabled {
		configJson, _ := json.Marshal(emailConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "email",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
	if webhookConfig.Enabled {
		configJson, _ := json.Marshal(webhookConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "webhook",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
}

// NotifyServerOnline 发送服务器上线告警（由连接管理器在服务器从离线恢复上线时调用）
func (s *AlertService) NotifyServerOnline(serverID string) {
	if !utils.GetSettingBool("alert_server_online_enabled", false) {
		return
	}
	cacheKey := fmt.Sprintf("alert_cooldown:%s:server_online", serverID)
	if facades.Cache().Get(cacheKey) != nil {
		return
	}
	_ = facades.Cache().Put(cacheKey, true, 2*time.Minute)

	emailConfig, webhookConfig, err := s.getNotificationConfigs(serverID)
	if err != nil || (!emailConfig.Enabled && !webhookConfig.Enabled) {
		return
	}

	serverRepo := repositories.GetServerRepository()
	server, _ := serverRepo.GetByID(serverID)
	serverName, serverIP := serverID, "未知"
	if server != nil {
		serverName, serverIP = server.Name, server.IP
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[恢复] %s - 服务器已上线", serverName)
	content := fmt.Sprintf("✅ 服务器已上线\n\n服务器: %s (%s)\n上线时间: %s",
		serverName, serverIP, timestamp)

	if emailConfig.Enabled {
		configJson, _ := json.Marshal(emailConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "email",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
	if webhookConfig.Enabled {
		configJson, _ := json.Marshal(webhookConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "webhook",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
}

// NotifyServiceMonitorProblem sends a notification when a service monitor enters a non-up state.
func (s *AlertService) NotifyServiceMonitorProblem(monitorID uint, status string, responseTime int, cause error) {
	cacheKey := fmt.Sprintf("alert_cooldown:service_monitor:%d:%s", monitorID, status)
	if facades.Cache().Get(cacheKey) != nil {
		return
	}
	_ = facades.Cache().Put(cacheKey, true, 2*time.Minute)

	monitor, err := repositories.GetServiceMonitorRepository().GetByID(monitorID)
	if err != nil || monitor == nil {
		return
	}

	emailConfig, webhookConfig, err := s.getNotificationConfigs("")
	if err != nil || (!emailConfig.Enabled && !webhookConfig.Enabled) {
		return
	}

	statusText := map[string]string{
		"down": "故障",
		"slow": "响应慢",
	}[status]
	if statusText == "" {
		statusText = status
	}

	reason := ""
	if cause != nil {
		reason = fmt.Sprintf("\n原因: %v", cause)
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[告警] %s - 服务%s", monitor.Name, statusText)
	content := fmt.Sprintf("服务监测告警\n\n服务: %s\n类型: %s\n目标: %s\n状态: %s\n响应时间: %dms\n触发时间: %s%s",
		monitor.Name, monitor.Type, monitor.Target, statusText, responseTime, timestamp, reason)

	if emailConfig.Enabled {
		configJson, _ := json.Marshal(emailConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "email",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
	if webhookConfig.Enabled {
		configJson, _ := json.Marshal(webhookConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "webhook",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
}

// NotifyServiceMonitorRecovery sends a notification when a service monitor returns to up.
func (s *AlertService) NotifyServiceMonitorRecovery(monitorID uint, previousStatus string, responseTime int) {
	cacheKey := fmt.Sprintf("alert_cooldown:service_monitor:%d:recovery", monitorID)
	if facades.Cache().Get(cacheKey) != nil {
		return
	}
	_ = facades.Cache().Put(cacheKey, true, 2*time.Minute)

	monitor, err := repositories.GetServiceMonitorRepository().GetByID(monitorID)
	if err != nil || monitor == nil {
		return
	}

	emailConfig, webhookConfig, err := s.getNotificationConfigs("")
	if err != nil || (!emailConfig.Enabled && !webhookConfig.Enabled) {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[恢复] %s - 服务已恢复", monitor.Name)
	content := fmt.Sprintf("服务监测恢复\n\n服务: %s\n类型: %s\n目标: %s\n上一状态: %s\n当前状态: 正常\n响应时间: %dms\n恢复时间: %s",
		monitor.Name, monitor.Type, monitor.Target, previousStatus, responseTime, timestamp)

	if emailConfig.Enabled {
		configJson, _ := json.Marshal(emailConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "email",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
	if webhookConfig.Enabled {
		configJson, _ := json.Marshal(webhookConfig)
		_ = facades.Queue().Job(&jobs.SendAlertJob{
			Channel: "webhook",
			Config:  string(configJson),
			Subject: title,
			Content: content,
		}).Dispatch()
	}
}
