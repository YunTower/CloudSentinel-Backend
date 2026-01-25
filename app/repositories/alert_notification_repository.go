package repositories

import (
	"encoding/json"
	"strings"

	"goravel/app/models"
	"goravel/app/services"

	"github.com/goravel/framework/facades"
)

// AlertNotificationRepository 告警通知
type AlertNotificationRepository struct{}

// NewAlertNotificationRepository 创建告警通知实例
func NewAlertNotificationRepository() *AlertNotificationRepository {
	return &AlertNotificationRepository{}
}

// GetByType 根据类型获取通知配置
func (r *AlertNotificationRepository) GetByType(notificationType string) (*models.AlertNotification, error) {
	var notification models.AlertNotification
	err := facades.Orm().Query().Where("notification_type", notificationType).First(&notification)
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

// GetAll 获取所有通知配置
func (r *AlertNotificationRepository) GetAll() ([]*models.AlertNotification, error) {
	var notifications []*models.AlertNotification
	err := facades.Orm().Query().Get(&notifications)
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

// UpdateConfig 更新通知配置
func (r *AlertNotificationRepository) UpdateConfig(notificationType string, config map[string]interface{}) error {
	if notificationType == "email" {
		if v, ok := config["password"].(string); ok && v != "" {
			if !strings.HasPrefix(v, "enc:") {
				if enc, err := services.EncryptStringWithAppKey(v); err == nil {
					config["password"] = enc
				}
			}
		}
	}
	if notificationType == "webhook" {
		if v, ok := config["webhook"].(string); ok && v != "" {
			if !strings.HasPrefix(v, "enc:") {
				if enc, err := services.EncryptStringWithAppKey(v); err == nil {
					config["webhook"] = enc
				}
			}
		}
	}
	configJson, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var notification models.AlertNotification
	err = facades.Orm().Query().Where("notification_type", notificationType).First(&notification)
	if err != nil {
		// 不存在则创建
		notification = models.AlertNotification{
			NotificationType: notificationType,
			Enabled:          true,
			ConfigJson:       string(configJson),
		}
		return facades.Orm().Query().Create(&notification)
	}

	// 存在则更新
	notification.ConfigJson = string(configJson)
	notification.Enabled = true
	return facades.Orm().Query().Save(&notification)
}

// SetEnabled 设置启用状态
func (r *AlertNotificationRepository) SetEnabled(notificationType string, enabled bool) error {
	var notification models.AlertNotification
	err := facades.Orm().Query().Where("notification_type", notificationType).First(&notification)
	if err != nil {
		return err
	}
	notification.Enabled = enabled
	return facades.Orm().Query().Save(&notification)
}
