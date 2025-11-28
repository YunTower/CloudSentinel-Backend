package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// AlertNotification 告警通知配置模型
type AlertNotification struct {
	ID               uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	NotificationType string    `gorm:"column:notification_type;not null;size:20" json:"notification_type"`
	Enabled          bool      `gorm:"column:enabled;default:0" json:"enabled"`
	ConfigJson       string    `gorm:"column:config_json;type:text;not null" json:"config_json"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at" json:"updated_at"`

	orm.Model
}

// TableName 指定表名
func (a *AlertNotification) TableName() string {
	return "alert_notifications"
}
