package utils

import (
	"github.com/goravel/framework/facades"
)

// GetSetting 获取系统设置值
func GetSetting(key string, defaultValue string) string {
	var value string
	if err := facades.DB().Table("system_settings").Where("setting_key", key).Value("setting_value", &value); err != nil {
		return defaultValue
	}
	if value == "" {
		return defaultValue
	}
	return value
}

// GetSettingBool 获取布尔类型的系统设置值
func GetSettingBool(key string, defaultValue bool) bool {
	value := GetSetting(key, "")
	if value == "" {
		return defaultValue
	}
	return value == "true"
}

// GetSettings 批量获取系统设置值
func GetSettings(keys []string) map[string]string {
	result := make(map[string]string)
	if len(keys) == 0 {
		return result
	}

	var settings []map[string]interface{}
	keysInterface := make([]interface{}, len(keys))
	for i, k := range keys {
		keysInterface[i] = k
	}

	err := facades.Orm().Query().Table("system_settings").
		Select("setting_key", "setting_value").
		WhereIn("setting_key", keysInterface).
		Get(&settings)

	if err != nil {
		return result
	}

	for _, setting := range settings {
		if key, ok := setting["setting_key"].(string); ok {
			if value, ok := setting["setting_value"].(string); ok {
				result[key] = value
			}
		}
	}

	return result
}

