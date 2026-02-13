package migrations

import (
	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

type AddLogRetentionSetting struct {
}

func (r *AddLogRetentionSetting) Signature() string {
	return "20260205000002_add_log_retention_setting"
}

func (r *AddLogRetentionSetting) Up() error {
	// 插入默认设置
	setting := models.SystemSetting{
		SettingKey:   "log_retention_days",
		SettingValue: "30",
		SettingType:  "int",
		Description:  "Agent日志保留天数",
	}
	err := facades.Orm().Query().Create(&setting)
	return err
}

func (r *AddLogRetentionSetting) Down() error {
	_, err := facades.Orm().Query().Where("setting_key", "log_retention_days").Delete(&models.SystemSetting{})
	return err
}
