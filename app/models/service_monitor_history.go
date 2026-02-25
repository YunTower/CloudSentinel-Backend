package models

import "time"

// ServiceMonitorHistory records the result of each individual check.
// status: "up" (green), "slow" (yellow – timed out but connection established), "down" (red – no connection)
type ServiceMonitorHistory struct {
	ID           uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	MonitorID    uint      `gorm:"column:monitor_id;not null;index" json:"monitor_id"`
	Status       string    `gorm:"column:status;not null" json:"status"`                // up, slow, down
	ResponseTime int       `gorm:"column:response_time;default:0" json:"response_time"` // ms
	CheckedAt    time.Time `gorm:"column:checked_at;not null" json:"checked_at"`
}

func (s *ServiceMonitorHistory) TableName() string {
	return "service_monitor_history"
}
