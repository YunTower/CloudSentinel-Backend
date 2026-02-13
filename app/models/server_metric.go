package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerMetric 服务器性能指标模型
type ServerMetric struct {
	ID              uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID        string    `gorm:"column:server_id;not null;size:255;index" json:"server_id"`
	CPUUsage        float64   `gorm:"column:cpu_usage;type:decimal(5,2);not null" json:"cpu_usage"`
	MemoryUsage     float64   `gorm:"column:memory_usage;type:decimal(5,2);not null" json:"memory_usage"`
	DiskUsage       float64   `gorm:"column:disk_usage;type:decimal(5,2);not null" json:"disk_usage"`
	NetworkUpload   float64   `gorm:"column:network_upload;type:decimal(10,2);default:0" json:"network_upload"`
	NetworkDownload float64   `gorm:"column:network_download;type:decimal(10,2);default:0" json:"network_download"`
	Uptime          string    `gorm:"column:uptime;size:100" json:"uptime"`
	Timestamp       time.Time `gorm:"column:timestamp;index" json:"timestamp"`

	// 非数据库字段，用于WebSocket传输
	SwapUsage      float64 `gorm:"-" json:"swap_usage"`
	DiskReadSpeed  float64 `gorm:"-" json:"disk_read_speed"`
	DiskWriteSpeed float64 `gorm:"-" json:"disk_write_speed"`
	Temperature    float64 `gorm:"-" json:"temperature"`

	orm.Model
}

// TableName 指定表名
func (s *ServerMetric) TableName() string {
	return "server_metrics"
}

// Server 关联的服务器
type ServerMetricWithServer struct {
	ServerMetric
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}
