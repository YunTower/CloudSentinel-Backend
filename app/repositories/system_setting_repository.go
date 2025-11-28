package repositories

import (
	"encoding/json"

	"goravel/app/models"

	"github.com/goravel/framework/facades"
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
	keysInterface := make([]interface{}, len(keys))
	for i, k := range keys {
		keysInterface[i] = k
	}

	err := facades.Orm().Query().WhereIn("setting_key", keysInterface).Get(&settings)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*models.SystemSetting)
	for i := range settings {
		result[settings[i].SettingKey] = &settings[i]
	}

	return result, nil
}

// SetValue 设置值
func (r *SystemSettingRepository) SetValue(key, value string) error {
	var setting models.SystemSetting
	err := facades.Orm().Query().Where("setting_key", key).First(&setting)

	if err != nil {
		// 不存在则创建
		setting = models.SystemSetting{
			SettingKey:   key,
			SettingValue: value,
			SettingType:  "string",
		}
		return facades.Orm().Query().Create(&setting)
	}

	// 存在则更新
	setting.SettingValue = value
	return facades.Orm().Query().Save(&setting)
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
