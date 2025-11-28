package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

type Server struct {
	ID                  string                 `gorm:"column:id;primaryKey" json:"id"`
	Name                string                 `gorm:"column:name" json:"name"`
	IP                  string                 `gorm:"column:ip" json:"ip"`
	Status              string                 `gorm:"column:status" json:"status"`
	OS                  string                 `gorm:"column:os" json:"os"`
	Architecture        string                 `gorm:"column:architecture" json:"architecture"`
	Kernel              string                 `gorm:"column:kernel" json:"kernel"`
	Hostname            string                 `gorm:"column:hostname" json:"hostname"`
	AgentKey            string                 `gorm:"column:agent_key" json:"agent_key"`
	AgentVersion        string                 `gorm:"column:agent_version" json:"agent_version"`
	SystemName          string                 `gorm:"column:system_name" json:"system_name"`
	BootTime            *time.Time             `gorm:"column:boot_time" json:"boot_time"`
	LastReportTime      *time.Time             `gorm:"column:last_report_time" json:"last_report_time"`
	UptimeDays          int                    `gorm:"column:uptime_days" json:"uptime_days"`
	Cores               int                    `gorm:"column:cores" json:"cores"`
	CreatedAt           time.Time              `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           time.Time              `gorm:"column:updated_at" json:"updated_at"`
	ServerMetrics       []*ServerMetric        `gorm:"foreignKey:ServerID;references:ID" json:"server_metrics,omitempty"`
	ServerDisks         []*ServerDisk          `gorm:"foreignKey:ServerID;references:ID" json:"server_disks,omitempty"`
	ServerMemoryHistory []*ServerMemoryHistory `gorm:"foreignKey:ServerID;references:ID" json:"server_memory_history,omitempty"`
	ServerSwap          *ServerSwap            `gorm:"foreignKey:ServerID;references:ID" json:"server_swap,omitempty"`
	ServerNetworkSpeed  []*ServerNetworkSpeed  `gorm:"foreignKey:ServerID;references:ID" json:"server_network_speed,omitempty"`

	orm.Model
}

func (s *Server) TableName() string {
	return "servers"
}

// GetLatestMetrics 获取最新指标
func (s *Server) GetLatestMetrics() *ServerMetric {
	if len(s.ServerMetrics) == 0 {
		return nil
	}
	// 返回最新的指标
	return s.ServerMetrics[0]
}

// GetTotalStorage 计算总存储容量
func (s *Server) GetTotalStorage() int64 {
	var total int64
	for _, disk := range s.ServerDisks {
		total += disk.TotalSize
	}
	return total
}

// GetDisks 获取磁盘列表
func (s *Server) GetDisks() []*ServerDisk {
	return s.ServerDisks
}
