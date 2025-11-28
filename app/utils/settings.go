package utils

import (
	"goravel/app/repositories"
)

// GetSetting 获取系统设置值
func GetSetting(key string, defaultValue string) string {
	return repositories.GetSystemSettingRepository().GetValue(key, defaultValue)
}

// GetSettingBool 获取布尔类型的系统设置值
func GetSettingBool(key string, defaultValue bool) bool {
	return repositories.GetSystemSettingRepository().GetBool(key, defaultValue)
}

// GetSettings 批量获取系统设置值
func GetSettings(keys []string) map[string]string {
	result := make(map[string]string)
	if len(keys) == 0 {
		return result
	}

	settings, err := repositories.GetSystemSettingRepository().GetByKeys(keys)
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
