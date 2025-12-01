package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

type Server struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	Name           string     `gorm:"column:name" json:"name"`
	IP             string     `gorm:"column:ip" json:"ip"`
	Status         string     `gorm:"column:status" json:"status"`
	OS             string     `gorm:"column:os" json:"os"`
	Architecture   string     `gorm:"column:architecture" json:"architecture"`
	Kernel         string     `gorm:"column:kernel" json:"kernel"`
	Hostname       string     `gorm:"column:hostname" json:"hostname"`
	AgentKey       string     `gorm:"column:agent_key" json:"agent_key"`
	AgentVersion   string     `gorm:"column:agent_version" json:"agent_version"`
	SystemName     string     `gorm:"column:system_name" json:"system_name"`
	BootTime       *time.Time `gorm:"column:boot_time" json:"boot_time"`
	LastReportTime *time.Time `gorm:"column:last_report_time" json:"last_report_time"`
	UptimeDays     int        `gorm:"column:uptime_days" json:"uptime_days"`
	Cores          int        `gorm:"column:cores" json:"cores"`
	// 分组和付费相关字段
	GroupID                *uint      `gorm:"column:group_id" json:"group_id"`
	BillingCycle           string     `gorm:"column:billing_cycle;size:20" json:"billing_cycle"`
	CustomCycleDays        *int       `gorm:"column:custom_cycle_days" json:"custom_cycle_days"`
	Price                  *float64   `gorm:"column:price" json:"price"`
	ExpireTime             *time.Time `gorm:"column:expire_time" json:"expire_time"`
	BandwidthMbps          int        `gorm:"column:bandwidth_mbps;default:0" json:"bandwidth_mbps"`
	TrafficLimitType       string     `gorm:"column:traffic_limit_type;size:20" json:"traffic_limit_type"`
	TrafficLimitBytes      int64      `gorm:"column:traffic_limit_bytes;default:0" json:"traffic_limit_bytes"`
	TrafficResetCycle      string     `gorm:"column:traffic_reset_cycle;size:20" json:"traffic_reset_cycle"`
	TrafficCustomCycleDays *int       `gorm:"column:traffic_custom_cycle_days" json:"traffic_custom_cycle_days"`
	CreatedAt              time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at" json:"updated_at"`
	// 关联关系
	ServerGroup         *ServerGroup           `gorm:"foreignKey:GroupID;references:ID" json:"server_group,omitempty"`
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
