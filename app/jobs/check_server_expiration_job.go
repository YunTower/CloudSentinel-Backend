package jobs

import (
	"encoding/json"
	"fmt"
	"goravel/app/repositories"
	"goravel/app/utils/notification"
	"time"

	"goravel/app/facades"
)

type CheckServerExpirationJob struct {
}

// Signature The name and signature of the job.
func (receiver *CheckServerExpirationJob) Signature() string {
	return "check_server_expiration_job"
}

// Handle Execute the job.
func (receiver *CheckServerExpirationJob) Handle(args ...any) error {
	facades.Log().Info("开始检查服务器到期告警")

	// 获取所有服务器
	serverRepo := repositories.GetServerRepository()
	servers, err := serverRepo.GetAll()
	if err != nil {
		facades.Log().Errorf("获取服务器列表失败: %v", err)
		return err
	}

	ruleRepo := repositories.GetServerAlertRuleRepository()
	notificationRepo := repositories.GetAlertNotificationRepository()

	// 获取通知配置
	emailConfig, webhookConfig := receiver.getNotificationConfigs(notificationRepo)

	// 检查每个服务器的到期时间
	for _, server := range servers {
		if server.ExpireTime == nil {
			continue
		}

		serverIDPtr := &server.ID
		// 先尝试获取服务器特定规则
		rule, err := ruleRepo.GetByServerIDAndType(serverIDPtr, "expiration")
		if err != nil {
			// 如果不存在，尝试获取全局规则
			rule, err = ruleRepo.GetByServerIDAndType(nil, "expiration")
			if err != nil {
				// 没有配置规则，跳过
				continue
			}
		}

		var config map[string]interface{}
		if err := json.Unmarshal([]byte(rule.Config), &config); err != nil {
			continue
		}

		enabled, _ := config["enabled"].(bool)
		if !enabled {
			continue
		}

		alertDays, ok := config["alert_days"].(float64)
		if !ok {
			continue
		}

		now := time.Now()
		expireTime := *server.ExpireTime
		daysUntilExpire := expireTime.Sub(now).Hours() / 24

		if daysUntilExpire <= alertDays && daysUntilExpire >= 0 {
			// 检查冷却期（每天只发送一次）
			cacheKey := fmt.Sprintf("alert_cooldown:%s:expiration", server.ID)
			if cooldown := facades.Cache().Get(cacheKey); cooldown != nil {
				continue
			}
			facades.Cache().Put(cacheKey, true, 24*time.Hour)

			// 触发告警
			title := fmt.Sprintf("[告警] %s - 即将到期", server.Name)
			rendered := notification.RenderConfiguredAlert(notification.AlertTemplateData{
				Event: "expiration", Status: "alert", Severity: "warning", Title: title,
				Summary: "服务器即将到期，请及时续期。", ResourceName: server.Name, ResourceType: "server", ResourceAddress: server.IP,
				OccurredAt: now.Format("2006-01-02 15:04:05"), Color: "#faad14",
				Fields: []notification.AlertTemplateField{{Label: "到期时间", Value: expireTime.Format("2006-01-02 15:04:05")}, {Label: "剩余天数", Value: fmt.Sprintf("%.0f 天", daysUntilExpire)}},
			})

			// 发送通知
			if enabled, ok := emailConfig["enabled"].(bool); ok && enabled {
				configJson, _ := json.Marshal(emailConfig)
				_ = facades.Queue().Job(&SendAlertJob{
					Channel: "email",
					Config:  string(configJson),
					Subject: rendered.EmailSubject,
					Content: rendered.EmailHTML,
				}).Dispatch()
			}
			if enabled, ok := webhookConfig["enabled"].(bool); ok && enabled {
				configJson, _ := json.Marshal(webhookConfig)
				_ = facades.Queue().Job(&SendAlertJob{
					Channel: "webhook",
					Config:  string(configJson),
					Subject: rendered.EmailSubject,
					Content: rendered.WebhookText,
				}).Dispatch()
			}
		}
	}

	facades.Log().Info("服务器到期告警检查完成")
	return nil
}

// getNotificationConfigs 获取通知配置（避免导入 services 包）
func (receiver *CheckServerExpirationJob) getNotificationConfigs(notificationRepo *repositories.AlertNotificationRepository) (map[string]interface{}, map[string]interface{}) {
	emailConfig := map[string]interface{}{"enabled": false}
	webhookConfig := map[string]interface{}{"enabled": false}

	notifications, err := notificationRepo.GetAll()
	if err != nil {
		return emailConfig, webhookConfig
	}

	// 解析配置
	for _, notif := range notifications {
		if !notif.Enabled || notif.ConfigJson == "" {
			continue
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(notif.ConfigJson), &cfg); err != nil {
			continue
		}

		switch notif.NotificationType {
		case "email":
			emailConfig = cfg
			emailConfig["enabled"] = true
		case "webhook":
			webhookConfig = cfg
			webhookConfig["enabled"] = true
		}
	}

	return emailConfig, webhookConfig
}
