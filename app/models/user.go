package models

import (
	"time"
)

// User 用户模型
type User struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Username  string    `json:"username"`
	Type      string    `json:"type"`
	IP        string    `json:"ip,omitempty"`
	UA        string    `json:"ua,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (u *User) TableName() string {
	return "users" // 使用标准的 users 表
}

// GetID 获取用户ID
func (u *User) GetID() any {
	return u.ID
}

// GetAuthIdentifierName 获取认证标识符名称
func (u *User) GetAuthIdentifierName() string {
	return "id"
}

// GetAuthIdentifier 获取认证标识符
func (u *User) GetAuthIdentifier() any {
	return u.ID
}

// GetAuthPassword 获取认证密码（这里不需要实现）
func (u *User) GetAuthPassword() string {
	return ""
}
