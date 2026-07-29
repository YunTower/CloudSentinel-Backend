package models

import (
	"strconv"
	"time"
)

// SystemSetting 系统设置模型
// 注意：不要嵌入 orm.Model，避免与本表 ID/时间戳字段冲突导致 Save 丢 key。
type SystemSetting struct {
	ID           uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SettingKey   string    `gorm:"column:setting_key;uniqueIndex;not null;size:100" json:"setting_key"`
	SettingValue string    `gorm:"column:setting_value;type:text" json:"setting_value"`
	SettingType  string    `gorm:"column:setting_type;default:string;size:20" json:"setting_type"`
	Description  string    `gorm:"column:description;type:text" json:"description"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
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
