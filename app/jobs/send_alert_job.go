package jobs

import (
	"encoding/json"
	"fmt"
	"goravel/app/services"
	"goravel/app/utils/notification"

	"github.com/goravel/framework/facades"
)

type SendAlertJob struct {
	Channel string
	Config  string
	Subject string
	Content string
}

// Signature The name and signature of the job.
func (receiver *SendAlertJob) Signature() string {
	return "send_alert_job"
}

// Handle Execute the job.
func (receiver *SendAlertJob) Handle(args ...any) error {
	facades.Log().Infof("Processing SendAlertJob: %s", receiver.Channel)

	switch receiver.Channel {
	case "email":
		var config notification.EmailConfig
		if err := json.Unmarshal([]byte(receiver.Config), &config); err != nil {
			return err
		}
		// 解密敏感字段
		if config.Password != "" {
			if dec, err := services.DecryptStringWithAppKey(config.Password); err == nil {
				config.Password = dec
			}
		}
		return notification.SendEmail(config, receiver.Subject, receiver.Content)
	case "webhook":
		var config notification.WebhookConfig
		if err := json.Unmarshal([]byte(receiver.Config), &config); err != nil {
			return err
		}
		// 解密敏感字段
		if config.Webhook != "" {
			if dec, err := services.DecryptStringWithAppKey(config.Webhook); err == nil {
				config.Webhook = dec
			}
		}
		return notification.SendWebhook(config, receiver.Content)
	default:
		return fmt.Errorf("unknown channel: %s", receiver.Channel)
	}
}
