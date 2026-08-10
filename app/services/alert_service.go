package services

import (
	"encoding/json"
	"fmt"
	"goravel/app/jobs"
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/utils"
	"goravel/app/utils/notification"
	"time"

	"goravel/app/facades"
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
	var title string
	var threshold float64
	if severity == "警告" {
		threshold = rule.Warning
	} else {
		threshold = rule.Critical
	}

	if isRecovery {
		title = fmt.Sprintf("[恢复] %s - %s", serverName, metricLabel)
	} else {
		title = fmt.Sprintf("[%s] %s - %s", severity, serverName, metricLabel)
	}

	color := "#ff4d4f" // 红色
	if severity == "警告" {
		color = "#faad14" // 橙色
	}
	if isRecovery {
		color = "#52c41a" // 绿色
	}

	status, statusText, summary := "alert", severity, fmt.Sprintf("%s已超过配置的告警阈值。", metricLabel)
	if isRecovery {
		status = "recovery"
		statusText = "恢复正常"
		summary = fmt.Sprintf("%s已恢复到正常范围。", metricLabel)
	}
	fields := []notification.AlertTemplateField{
		{Label: "指标", Value: metricLabel},
		{Label: "状态", Value: statusText},
		{Label: "当前值", Value: fmt.Sprintf("%.2f%%", value)},
	}
	if !isRecovery {
		fields = append(fields, notification.AlertTemplateField{Label: "触发阈值", Value: fmt.Sprintf("%.2f%%", threshold)})
	}
	s.dispatchAlert(serverID, notification.AlertTemplateData{
		Event: metricName, Status: status, Severity: severity, Title: title, Summary: summary,
		ResourceName: serverName, ResourceType: "server", ResourceAddress: serverIP,
		OccurredAt: timestamp, Color: color, Fields: fields,
	})
}

// dispatchAlert 统一完成模板渲染与渠道任务分发，业务方法只负责构造告警语义。
func (s *AlertService) dispatchAlert(serverID string, data notification.AlertTemplateData) {
	rendered := notification.RenderConfiguredAlert(data)
	emailConfig, webhookConfig, err := s.getNotificationConfigs(serverID)
	if err != nil {
		facades.Log().Warningf("获取通知配置失败: %v", err)
		return
	}
	if emailConfig.Enabled {
		configJSON, _ := json.Marshal(emailConfig)
		if err := facades.Queue().Job(&jobs.SendAlertJob{Channel: "email", Config: string(configJSON), Subject: rendered.EmailSubject, Content: rendered.EmailHTML}).Dispatch(); err != nil {
			facades.Log().Errorf("分发邮件发送任务失败: %v", err)
		}
	}
	if webhookConfig.Enabled {
		configJSON, _ := json.Marshal(webhookConfig)
		if err := facades.Queue().Job(&jobs.SendAlertJob{Channel: "webhook", Config: string(configJSON), Subject: rendered.EmailSubject, Content: rendered.WebhookText}).Dispatch(); err != nil {
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

		// 检查冷却期
		cacheKey := fmt.Sprintf("alert_cooldown:%s:bandwidth", serverID)
		if cooldown := facades.Cache().Get(cacheKey); cooldown != nil {
			return nil
		}
		facades.Cache().Put(cacheKey, true, 2*time.Minute)

		s.dispatchAlert(serverID, notification.AlertTemplateData{
			Event: "bandwidth", Status: "alert", Severity: "warning", Title: title,
			Summary: "当前带宽已达到配置的峰值阈值。", ResourceName: serverName, ResourceType: "server", ResourceAddress: serverIP,
			OccurredAt: time.Now().Format("2006-01-02 15:04:05"), Color: "#faad14",
			Fields: []notification.AlertTemplateField{{Label: "当前带宽", Value: fmt.Sprintf("%.2f Mbps", currentMbps)}, {Label: "触发阈值", Value: fmt.Sprintf("%.2f Mbps", threshold)}},
		})
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

		// 检查冷却期
		cacheKey := fmt.Sprintf("alert_cooldown:%s:traffic", serverID)
		if cooldown := facades.Cache().Get(cacheKey); cooldown != nil {
			return nil
		}
		facades.Cache().Put(cacheKey, true, 2*time.Minute)

		s.dispatchAlert(serverID, notification.AlertTemplateData{
			Event: "traffic", Status: "alert", Severity: "warning", Title: title,
			Summary: "流量使用率已达到配置的告警阈值。", ResourceName: serverName, ResourceType: "server", ResourceAddress: serverIP,
			OccurredAt: time.Now().Format("2006-01-02 15:04:05"), Color: "#faad14",
			Fields: []notification.AlertTemplateField{{Label: "已用流量", Value: fmt.Sprintf("%.2f GB / %.2f GB (%.2f%%)", usedGB, limitGB, usedPercent)}, {Label: "触发阈值", Value: fmt.Sprintf("%.2f%%", thresholdPercent)}},
		})
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

		// 检查冷却期（每天只发送一次）
		cacheKey := fmt.Sprintf("alert_cooldown:%s:expiration", serverID)
		if cooldown := facades.Cache().Get(cacheKey); cooldown != nil {
			return nil
		}
		facades.Cache().Put(cacheKey, true, 24*time.Hour)

		s.dispatchAlert(serverID, notification.AlertTemplateData{
			Event: "expiration", Status: "alert", Severity: "warning", Title: title,
			Summary: "服务器即将到期，请及时续期。", ResourceName: server.Name, ResourceType: "server", ResourceAddress: server.IP,
			OccurredAt: now.Format("2006-01-02 15:04:05"), Color: "#faad14",
			Fields: []notification.AlertTemplateField{{Label: "到期时间", Value: expireTime.Format("2006-01-02 15:04:05")}, {Label: "剩余天数", Value: fmt.Sprintf("%.0f 天", daysUntilExpire)}},
		})
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

	serverRepo := repositories.GetServerRepository()
	server, _ := serverRepo.GetByID(serverID)
	serverName, serverIP := serverID, "未知"
	if server != nil {
		serverName, serverIP = server.Name, server.IP
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[告警] %s - 服务器离线", serverName)
	s.dispatchAlert(serverID, notification.AlertTemplateData{
		Event: "server.offline", Status: "alert", Severity: "critical", Title: title,
		Summary: "服务器 Agent 已断开连接，请检查服务器或 Agent 状态。", ResourceName: serverName, ResourceType: "server", ResourceAddress: serverIP,
		OccurredAt: timestamp, Color: "#d03050", Fields: []notification.AlertTemplateField{{Label: "状态", Value: "离线"}},
	})
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

	serverRepo := repositories.GetServerRepository()
	server, _ := serverRepo.GetByID(serverID)
	serverName, serverIP := serverID, "未知"
	if server != nil {
		serverName, serverIP = server.Name, server.IP
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[恢复] %s - 服务器已上线", serverName)
	s.dispatchAlert(serverID, notification.AlertTemplateData{
		Event: "server.online", Status: "recovery", Severity: "info", Title: title,
		Summary: "服务器 Agent 已恢复连接。", ResourceName: serverName, ResourceType: "server", ResourceAddress: serverIP,
		OccurredAt: timestamp, Color: "#18a058", Fields: []notification.AlertTemplateField{{Label: "状态", Value: "在线"}},
	})
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

	statusText := map[string]string{
		"down": "故障",
		"slow": "响应慢",
	}[status]
	if statusText == "" {
		statusText = status
	}

	reason := "未知"
	if cause != nil {
		reason = cause.Error()
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[告警] %s - 服务%s", monitor.Name, statusText)
	s.dispatchAlert("", notification.AlertTemplateData{
		Event: "service_monitor.problem", Status: "alert", Severity: status, Title: title,
		Summary: "服务监测发现异常。", ResourceName: monitor.Name, ResourceType: "service_monitor", ResourceAddress: monitor.Target,
		OccurredAt: timestamp, Color: "#d03050",
		Fields: []notification.AlertTemplateField{{Label: "类型", Value: monitor.Type}, {Label: "状态", Value: statusText}, {Label: "响应时间", Value: fmt.Sprintf("%dms", responseTime)}, {Label: "原因", Value: reason}},
	})
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

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	title := fmt.Sprintf("[恢复] %s - 服务已恢复", monitor.Name)
	s.dispatchAlert("", notification.AlertTemplateData{
		Event: "service_monitor.recovery", Status: "recovery", Severity: "info", Title: title,
		Summary: "服务监测已恢复正常。", ResourceName: monitor.Name, ResourceType: "service_monitor", ResourceAddress: monitor.Target,
		OccurredAt: timestamp, Color: "#18a058",
		Fields: []notification.AlertTemplateField{{Label: "类型", Value: monitor.Type}, {Label: "上一状态", Value: previousStatus}, {Label: "当前状态", Value: "正常"}, {Label: "响应时间", Value: fmt.Sprintf("%dms", responseTime)}},
	})
}
