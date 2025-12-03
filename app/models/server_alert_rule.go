package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerAlertRule 服务器告警规则模型
type ServerAlertRule struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID  *string   `gorm:"column:server_id;index;size:36" json:"server_id"` // NULL 表示全局规则
	RuleType  string    `gorm:"column:rule_type;not null;size:50;index" json:"rule_type"` // cpu, memory, disk, bandwidth, traffic, expiration
	Config    string    `gorm:"column:config;type:text;not null" json:"config"`             // JSON 格式配置
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`

	orm.Model
}

// TableName 指定表名
func (s *ServerAlertRule) TableName() string {
	return "server_alert_rules"
}


