package repositories

import (
	"encoding/json"
	"fmt"
	"time"

	"goravel/app/models"

	"goravel/app/facades"
)

// SystemSettingRepository 系统设置
type SystemSettingRepository struct{}

// NewSystemSettingRepository 创建系统设置实例
func NewSystemSettingRepository() *SystemSettingRepository {
	return &SystemSettingRepository{}
}

// GetByKey 根据键获取设置
func (r *SystemSettingRepository) GetByKey(key string) (*models.SystemSetting, error) {
	var setting models.SystemSetting
	err := facades.Orm().Query().Where("setting_key", key).First(&setting)
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// GetByKeys 批量获取设置
func (r *SystemSettingRepository) GetByKeys(keys []string) (map[string]*models.SystemSetting, error) {
	if len(keys) == 0 {
		return make(map[string]*models.SystemSetting), nil
	}

	var settings []models.SystemSetting
	err := facades.Orm().Query().WhereIn("setting_key", stringsToInterfaceSlice(keys)).Get(&settings)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*models.SystemSetting)
	for i := range settings {
		result[settings[i].SettingKey] = &settings[i]
	}

	return result, nil
}

// SetValue 设置值（不存在则新建，存在则只更新 setting_value）
// 使用 Table + map 写入，避免模型 Save 因字段冲突把 setting_key 写成空字符串。
func (r *SystemSettingRepository) SetValue(key, value string) error {
	if key == "" {
		return fmt.Errorf("setting_key 不能为空")
	}

	count, err := facades.Orm().Query().Table("system_settings").Where("setting_key", key).Count()
	if err != nil {
		return err
	}

	now := time.Now()
	if count == 0 {
		row := map[string]any{
			"setting_key":   key,
			"setting_value": value,
			"setting_type":  "string",
			"created_at":    now,
			"updated_at":    now,
		}
		return facades.Orm().Query().Table("system_settings").Create(row)
	}

	_, err = facades.Orm().Query().Table("system_settings").
		Where("setting_key", key).
		Update(map[string]any{
			"setting_value": value,
			"updated_at":    now,
		})
	return err
}

// GetValue 获取值
func (r *SystemSettingRepository) GetValue(key, defaultValue string) string {
	setting, err := r.GetByKey(key)
	if err != nil || setting == nil {
		return defaultValue
	}
	value := setting.GetValue()
	if value == "" {
		return defaultValue
	}
	return value
}

// GetBool 获取布尔值
func (r *SystemSettingRepository) GetBool(key string, defaultValue bool) bool {
	setting, err := r.GetByKey(key)
	if err != nil || setting == nil {
		return defaultValue
	}
	return setting.GetBool()
}

// GetInt 获取整数值
func (r *SystemSettingRepository) GetInt(key string, defaultValue int) int {
	setting, err := r.GetByKey(key)
	if err != nil || setting == nil {
		return defaultValue
	}
	val := setting.GetInt()
	if val == 0 && setting.GetValue() != "0" {
		return defaultValue
	}
	return val
}

// GetJSON 获取 JSON 值并解析
func (r *SystemSettingRepository) GetJSON(key string, target interface{}) error {
	setting, err := r.GetByKey(key)
	if err != nil || setting == nil {
		return err
	}
	return json.Unmarshal([]byte(setting.GetValue()), target)
}

// SetJSON 设置 JSON 值
func (r *SystemSettingRepository) SetJSON(key string, value interface{}) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.SetValue(key, string(jsonData))
}

// GetJSONWithDefault 获取 JSON 值，如果不存在则返回默认值
func (r *SystemSettingRepository) GetJSONWithDefault(key string, target interface{}, defaultValue interface{}) error {
	setting, err := r.GetByKey(key)
	if err != nil || setting == nil {
		// 如果不存在，使用默认值
		if defaultValue != nil {
			jsonData, err := json.Marshal(defaultValue)
			if err == nil {
				return json.Unmarshal(jsonData, target)
			}
		}
		return err
	}
	if setting.GetValue() == "" {
		// 如果值为空，使用默认值
		if defaultValue != nil {
			jsonData, err := json.Marshal(defaultValue)
			if err == nil {
				return json.Unmarshal(jsonData, target)
			}
		}
		return nil
	}
	return json.Unmarshal([]byte(setting.GetValue()), target)
}
