package models

import (
	"strconv"
	"time"

	"github.com/goravel/framework/database/orm"
)

// SystemSetting 系统设置模型
type SystemSetting struct {
	ID           uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SettingKey   string    `gorm:"column:setting_key;uniqueIndex;not null;size:100" json:"setting_key"`
	SettingValue string    `gorm:"column:setting_value;type:text" json:"setting_value"`
	SettingType  string    `gorm:"column:setting_type;default:string;size:20" json:"setting_type"`
	Description  string    `gorm:"column:description;type:text" json:"description"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`

	orm.Model
}

// TableName 指定表名
func (s *SystemSetting) TableName() string {
	return "system_settings"
}

// GetValue 获取设置值（字符串）
func (s *SystemSetting) GetValue() string {
	return s.SettingValue
}

// GetBool 获取设置值（布尔类型）
func (s *SystemSetting) GetBool() bool {
	return s.SettingValue == "true"
}

// GetInt 获取设置值（整数类型）
func (s *SystemSetting) GetInt() int {
	val, _ := strconv.Atoi(s.SettingValue)
	return val
}

// SetValue 设置值
func (s *SystemSetting) SetValue(value string) {
	s.SettingValue = value
}
