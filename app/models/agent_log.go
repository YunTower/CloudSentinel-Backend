package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

type AgentLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ServerID  string    `gorm:"index" json:"server_id"`
	Level     string    `gorm:"size:20" json:"level"`
	Message   string    `gorm:"type:text" json:"message"`
	Context   string    `gorm:"type:text" json:"context"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`

	orm.Model
}

func (r *AgentLog) TableName() string {
	return "agent_logs"
}
