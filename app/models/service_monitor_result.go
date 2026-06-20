package models

import "time"

// ServiceMonitorResult stores the raw result of one service check.
type ServiceMonitorResult struct {
	ID            uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	MonitorID     uint      `gorm:"column:monitor_id;not null;index" json:"monitor_id"`
	ProbeType     string    `gorm:"column:probe_type;not null;size:30;index" json:"probe_type"` // panel, agent
	ProbeID       string    `gorm:"column:probe_id;size:100;index" json:"probe_id,omitempty"`
	ProbeName     string    `gorm:"column:probe_name;size:100" json:"probe_name,omitempty"`
	ProbeLocation string    `gorm:"column:probe_location;size:100" json:"probe_location,omitempty"`
	Status        string    `gorm:"column:status;not null;size:30;index" json:"status"` // up, slow, down
	ResponseTime  int       `gorm:"column:response_time;default:0" json:"response_time"`
	Error         string    `gorm:"column:error;type:text" json:"error,omitempty"`
	CheckedAt     time.Time `gorm:"column:checked_at;not null;index" json:"checked_at"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (s *ServiceMonitorResult) TableName() string {
	return "service_monitor_results"
}
