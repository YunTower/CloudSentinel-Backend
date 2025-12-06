package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerNotificationChannel 服务器通知渠道配置模型
type ServerNotificationChannel struct {
	ID               uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID         string    `gorm:"column:server_id;not null;size:36;index" json:"server_id"`
	NotificationType string    `gorm:"column:notification_type;not null;size:20;index" json:"notification_type"` // email, webhook
	Enabled          bool      `gorm:"column:enabled;default:0" json:"enabled"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at" json:"updated_at"`

	orm.Model
}

// TableName 指定表名
func (s *ServerNotificationChannel) TableName() string {
	return "server_notification_channels"
}

