package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerNetworkSpeed 服务器网络速度模型
type ServerNetworkSpeed struct {
	ID            uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID      string    `gorm:"column:server_id;not null;size:255;index" json:"server_id"`
	UploadSpeed   float64   `gorm:"column:upload_speed;type:decimal(10,2);not null" json:"upload_speed"`
	DownloadSpeed float64   `gorm:"column:download_speed;type:decimal(10,2);not null" json:"download_speed"`
	Timestamp     time.Time `gorm:"column:timestamp;index" json:"timestamp"`

	orm.Model
}

// TableName 指定表名
func (s *ServerNetworkSpeed) TableName() string {
	return "server_network_speed"
}

// Server 关联的服务器
type ServerNetworkSpeedWithServer struct {
	ServerNetworkSpeed
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}
