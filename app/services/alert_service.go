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

// alertNotifyCooldown 告警通知冷却期：持续告警重发与阈值抖动导致的
// 状态翻转共用，避免通知风暴；恢复通知不受冷却限制。
const alertNotifyCooldown = 2 * time.Minute

// alertStateRow 读取持久化的告警状态；状态机不再依赖进程内缓存，
// 重启后不会重复告警、也不会丢失恢复通知。
func (s *AlertService) alertStateRow(serverID, metric string) *models.AlertState {
	row, err := repositories.NewAlertStateRepository().Get(serverID, metric)
	if err != nil || row == nil {
		return nil
	}
	return row
}

// cooldownActive 判断该维度的告警通知是否仍在冷却期内。
func (s *AlertService) cooldownActive(row *models.AlertState) bool {
	return row != nil && row.LastNotifiedAt != nil && time.Since(*row.LastNotifiedAt) < alertNotifyCooldown
}

// notifyCooldownActive 供带宽/流量/到期/上下线等仅用冷却期的告警使用。
func (s *AlertService) notifyCooldownActive(serverID, scope string, ttl time.Duration) bool {
	row := s.alertStateRow(serverID, scope)
	if row != nil && row.LastNotifiedAt != nil && time.Since(*row.LastNotifiedAt) < ttl {
		return true
	}
	return false
}

// markNotified 记录最近一次通知时间（冷却期起点）。
func (s *AlertService) markNotified(serverID, metric, state string) {
	now := time.Now()
	if err := repositories.NewAlertStateRepository().Upsert(serverID, metric, state, &now); err != nil {
		facades.Log().Warningf("写入告警状态失败: server=%s metric=%s error=%v", serverID, metric, err)
	}
}

// saveAlertState 仅更新状态，不触碰冷却期。
func (s *AlertService) saveAlertState(serverID, metric, state string) {
	if err := repositories.NewAlertStateRepository().Upsert(serverID, metric, state, nil); err != nil {
		facades.Log().Warningf("写入告警状态失败: server=%s metric=%s error=%v", serverID, metric, err)
	}
}

// evaluateRule 评估单个规则
func (s *AlertService) evaluateRule(serverID, metricName string, value float64, rule Rule) error {
	if !rule.Enabled {
		return nil
	}

	row := s.alertStateRow(serverID, metricName)
	currentState := AlertStateNormal
	if row != nil && row.State != "" {
		currentState = AlertState(row.State)
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

	// 恢复通知立即发送，不进入冷却
	if newState == AlertStateNormal && currentState != AlertStateNormal {
		s.saveAlertState(serverID, metricName, string(newState))
		s.sendNotification(serverID, metricName, value, newState, severity, true, rule)
		return nil
	}

	// 状态未变化：正常态静默；告警态按冷却期决定是否重发
	if newState == currentState {
		if newState == AlertStateNormal {
			return nil
		}
		if s.cooldownActive(row) {
			return nil
		}
		s.markNotified(serverID, metricName, string(newState))
		s.sendNotification(serverID, metricName, value, newState, severity, false, rule)
		return nil
	}

	// 状态变化（如 warning→critical、normal→warning）：同样受冷却期约束，
	// 防止目标在阈值附近抖动时每次翻转都发通知。
	if s.cooldownActive(row) {
		s.saveAlertState(serverID, metricName, string(newState))
		return nil
	}
	s.markNotified(serverID, metricName, string(newState))
	s.sendNotification(serverID, metricName, value, newState, severity, false, rule)
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

// alertDispatchSem 限制并发告警发送 goroutine 数量。默认 QUEUE_CONNECTION=sync
// 时 Dispatch 会在调用方 goroutine 内同步执行 SMTP/Webhook（可达数十秒），
// 直接调用会阻塞数据采集 worker；包一层有界异步避免拖垮采集链路。
var alertDispatchSem = make(chan struct{}, 8)

// dispatchAlert 统一完成模板渲染与渠道任务分发，业务方法只负责构造告警语义。
func (s *AlertService) dispatchAlert(serverID string, data notification.AlertTemplateData) {
	rendered := notification.RenderConfiguredAlert(data)
	emailConfig, webhookConfig, err := s.getNotificationConfigs(serverID)
	if err != nil {
		facades.Log().Warningf("获取通知配置失败: %v", err)
		return
	}
	type dispatchTask struct {
		channel string
		config  string
		subject string
		content string
	}
	tasks := make([]dispatchTask, 0, 2)
	if emailConfig.Enabled {
		configJSON, _ := json.Marshal(emailConfig)
		tasks = append(tasks, dispatchTask{"email", string(configJSON), rendered.EmailSubject, rendered.EmailHTML})
	}
	if webhookConfig.Enabled {
		configJSON, _ := json.Marshal(webhookConfig)
		tasks = append(tasks, dispatchTask{"webhook", string(configJSON), rendered.EmailSubject, rendered.WebhookText})
	}
	for _, task := range tasks {
		task := task
		select {
		case alertDispatchSem <- struct{}{}:
			go func() {
				defer func() { <-alertDispatchSem }()
				if err := facades.Queue().Job(&jobs.SendAlertJob{Channel: task.channel, Config: task.config, Subject: task.subject, Content: task.content}).Dispatch(); err != nil {
					facades.Log().Errorf("分发%s发送任务失败: %v", task.channel, err)
				}
			}()
		default:
			// 发送通道已饱和：丢弃本次通知并记录，宁可丢通知也不能阻塞采集
			facades.Log().Warningf("告警发送通道饱和，丢弃通知: channel=%s event=%s", task.channel, data.Event)
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

		// 检查冷却期（持久化，重启不丢失）
		if s.notifyCooldownActive(serverID, "bandwidth", alertNotifyCooldown) {
			return nil
		}
		s.markNotified(serverID, "bandwidth", string(AlertStateWarning))

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

		// 检查冷却期（持久化，重启不丢失）
		if s.notifyCooldownActive(serverID, "traffic", alertNotifyCooldown) {
			return nil
		}
		s.markNotified(serverID, "traffic", string(AlertStateWarning))

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

		// 检查冷却期（每天只发送一次，持久化）
		if s.notifyCooldownActive(serverID, "expiration", 24*time.Hour) {
			return nil
		}
		s.markNotified(serverID, "expiration", string(AlertStateWarning))

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
	if s.notifyCooldownActive(serverID, "server_offline", 5*time.Minute) {
		return
	}
	s.markNotified(serverID, "server_offline", string(AlertStateCritical))

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
	if s.notifyCooldownActive(serverID, "server_online", alertNotifyCooldown) {
		return
	}
	s.markNotified(serverID, "server_online", string(AlertStateNormal))

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
	scope := fmt.Sprintf("service_monitor:%d:%s", monitorID, status)
	if s.notifyCooldownActive("", scope, alertNotifyCooldown) {
		return
	}
	s.markNotified("", scope, string(AlertStateCritical))

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
	scope := fmt.Sprintf("service_monitor:%d:recovery", monitorID)
	if s.notifyCooldownActive("", scope, alertNotifyCooldown) {
		return
	}
	s.markNotified("", scope, string(AlertStateNormal))

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
