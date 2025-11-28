package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerMemoryHistory 服务器内存历史记录模型
type ServerMemoryHistory struct {
	ID                 uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID           string    `gorm:"column:server_id;not null;size:255;index" json:"server_id"`
	MemoryTotal        int64     `gorm:"column:memory_total;not null" json:"memory_total"`
	MemoryUsed         int64     `gorm:"column:memory_used;not null" json:"memory_used"`
	MemoryUsagePercent float64   `gorm:"column:memory_usage_percent;type:decimal(5,2);not null" json:"memory_usage_percent"`
	Timestamp          time.Time `gorm:"column:timestamp;index" json:"timestamp"`

	orm.Model
}

// TableName 指定表名
func (s *ServerMemoryHistory) TableName() string {
	return "server_memory_history"
}

// Server 关联的服务器
type ServerMemoryHistoryWithServer struct {
	ServerMemoryHistory
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}
