package models

import (
	"github.com/goravel/framework/database/orm"
)

// User 用户模型
type User struct {
	orm.Model
	Username string `json:"username"`
	Type     string `json:"type"`
	IP       string `json:"ip,omitempty"`
	UA       string `json:"ua,omitempty"`
}

// TableName 指定表名
func (u *User) TableName() string {
	return "system_settings"
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
