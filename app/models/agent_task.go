package models

import "time"

type AgentTask struct {
	ID             string                 `gorm:"column:id;primaryKey" json:"id"`
	ServerID       string                 `gorm:"column:server_id;index" json:"server_id"`
	Command        string                 `gorm:"column:command;size:50" json:"command"`
	CommandID      string                 `gorm:"column:command_id;size:100" json:"command_id"`
	Payload        map[string]interface{} `gorm:"column:payload;serializer:json" json:"data"`
	Status         string                 `gorm:"column:status;size:30;index" json:"status"`
	Attempts       int                    `gorm:"column:attempts;default:0" json:"attempts"`
	Error          string                 `gorm:"column:error;type:text" json:"error,omitempty"`
	AvailableAt    *time.Time             `gorm:"column:available_at" json:"available_at,omitempty"`
	DeliveredAt    *time.Time             `gorm:"column:delivered_at" json:"delivered_at,omitempty"`
	LeaseToken     string                 `gorm:"column:lease_token;size:36;index" json:"lease_token,omitempty"`
	LeaseExpiresAt *time.Time             `gorm:"column:lease_expires_at" json:"lease_expires_at,omitempty"`
	CompletedAt    *time.Time             `gorm:"column:completed_at" json:"completed_at,omitempty"`
	CreatedAt      time.Time              `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time              `gorm:"column:updated_at" json:"updated_at"`
}

func (a *AgentTask) TableName() string {
	return "agent_tasks"
}
