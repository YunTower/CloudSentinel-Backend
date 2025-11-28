package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerSwap 服务器交换分区信息模型
type ServerSwap struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID  string    `gorm:"column:server_id;not null;size:255;index" json:"server_id"`
	SwapTotal int64     `gorm:"column:swap_total;not null" json:"swap_total"`
	SwapUsed  int64     `gorm:"column:swap_used;not null" json:"swap_used"`
	SwapFree  int64     `gorm:"column:swap_free;not null" json:"swap_free"`
	Timestamp time.Time `gorm:"column:timestamp;index" json:"timestamp"`

	orm.Model
}

// TableName 指定表名
func (s *ServerSwap) TableName() string {
	return "server_swap"
}

// Server 关联的服务器
type ServerSwapWithServer struct {
	ServerSwap
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}
