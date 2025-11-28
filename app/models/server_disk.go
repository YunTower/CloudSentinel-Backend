package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerDisk 服务器磁盘信息模型
type ServerDisk struct {
	ID         uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ServerID   string    `gorm:"column:server_id;not null;size:255;index" json:"server_id"`
	DiskName   string    `gorm:"column:disk_name;not null;size:100" json:"disk_name"`
	MountPoint string    `gorm:"column:mount_point;size:200" json:"mount_point"`
	Filesystem string    `gorm:"column:filesystem;size:50" json:"filesystem"`
	TotalSize  int64     `gorm:"column:total_size;not null" json:"total_size"`
	UsedSize   int64     `gorm:"column:used_size;default:0" json:"used_size"`
	FreeSize   int64     `gorm:"column:free_size;default:0" json:"free_size"`
	DiskType   string    `gorm:"column:disk_type;default:unknown;size:50" json:"disk_type"`
	IsBoot     bool      `gorm:"column:is_boot;default:0" json:"is_boot"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`

	orm.Model
}

// TableName 指定表名
func (s *ServerDisk) TableName() string {
	return "server_disks"
}

// Server 关联的服务器
type ServerDiskWithServer struct {
	ServerDisk
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}
