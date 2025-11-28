package utils

import (
	"goravel/app/repositories"
)

// GetSetting 获取系统设置值
func GetSetting(key string, defaultValue string) string {
	settingRepo := repositories.NewSystemSettingRepository()
	return settingRepo.GetValue(key, defaultValue)
}

// GetSettingBool 获取布尔类型的系统设置值
func GetSettingBool(key string, defaultValue bool) bool {
	settingRepo := repositories.NewSystemSettingRepository()
	return settingRepo.GetBool(key, defaultValue)
}

// GetSettings 批量获取系统设置值
func GetSettings(keys []string) map[string]string {
	result := make(map[string]string)
	if len(keys) == 0 {
		return result
	}

	settingRepo := repositories.NewSystemSettingRepository()
	settings, err := settingRepo.GetByKeys(keys)
	if err != nil {
		return result
	}

	for key, setting := range settings {
		if setting != nil {
			result[key] = setting.GetValue()
		}
	}

	return result
}
