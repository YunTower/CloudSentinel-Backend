package models

import (
	"time"
)

// AlertState 持久化的告警状态与通知时间。
// 进程重启后告警状态机和冷却期不丢失：缓存方案会导致持续超阈值在
// TTL 过期后重复告警、真实恢复事件丢失恢复通知。
type AlertState struct {
	ID             uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID       string     `gorm:"column:server_id;size:64" json:"server_id"` // 服务器ID或监测对象标识（可为空串）
	Metric         string     `gorm:"column:metric;size:64" json:"metric"`       // cpu/memory/disk/bandwidth/... 或 service_monitor:<id>
	State          string     `gorm:"column:state;size:20" json:"state"`         // normal/warning/critical
	LastNotifiedAt *time.Time `gorm:"column:last_notified_at" json:"last_notified_at"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (AlertState) TableName() string {
	return "alert_states"
}
