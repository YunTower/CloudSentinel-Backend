package migrations

import (
	"time"

	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

type AddUpdateChannelSetting struct{}

func (r *AddUpdateChannelSetting) Signature() string {
	return "20260221000001_add_update_channel_setting"
}

func (r *AddUpdateChannelSetting) Up() error {
	// 已存在则跳过
	var existing models.SystemSetting
	if facades.Orm().Query().Where("setting_key", "update_channel").First(&existing) == nil {
		return nil
	}
	now := time.Now().Unix()
	row := map[string]any{
		"setting_key":   "update_channel",
		"setting_value": "release",
		"setting_type":  "string",
		"description":   "更新渠道（release=正式版，beta=测试版，dev=开发版）",
		"created_at":    now,
		"updated_at":    now,
	}
	return facades.Orm().Query().Table("system_settings").Create(row)
}

func (r *AddUpdateChannelSetting) Down() error {
	_, err := facades.Orm().Query().Where("setting_key", "update_channel").Delete(&models.SystemSetting{})
	return err
}
