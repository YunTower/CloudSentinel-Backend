package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ServerGroup 服务器分组模型
type ServerGroup struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;not null;size:100" json:"name"`
	Description string    `gorm:"column:description;type:text" json:"description"`
	Color       string    `gorm:"column:color;size:20" json:"color"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`

	orm.Model
}

// TableName 指定表名
func (s *ServerGroup) TableName() string {
	return "server_groups"
}
