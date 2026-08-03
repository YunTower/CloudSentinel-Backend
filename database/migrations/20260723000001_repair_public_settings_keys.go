package migrations

import (
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

// M20260723000001RepairPublicSettingsKeys 修复 setting_key 为空的公开配置行，并补唯一索引。
type M20260723000001RepairPublicSettingsKeys struct{}

func (r *M20260723000001RepairPublicSettingsKeys) Signature() string {
	return "20260723000001_repair_public_settings_keys"
}

func (r *M20260723000001RepairPublicSettingsKeys) Up() error {
	type row struct {
		ID           uint   `gorm:"column:id"`
		SettingKey   string `gorm:"column:setting_key"`
		SettingValue string `gorm:"column:setting_value"`
	}

	var orphans []row
	_ = facades.Orm().Query().Table("system_settings").
		Where("setting_key = ? OR setting_key IS NULL", "").
		OrderBy("id", "desc").
		Get(&orphans)

	now := time.Now()
	assignIfMissing := func(targetKey, value string) {
		count, err := facades.Orm().Query().Table("system_settings").Where("setting_key", targetKey).Count()
		if err != nil || count > 0 || strings.TrimSpace(value) == "" {
			return
		}
		_ = facades.Orm().Query().Table("system_settings").Create(map[string]any{
			"setting_key":   targetKey,
			"setting_value": value,
			"setting_type":  "string",
			"created_at":    now,
			"updated_at":    now,
		})
	}

	for _, orphan := range orphans {
		v := orphan.SettingValue
		switch {
		case strings.Contains(v, `"pages"`) && strings.Contains(v, `"blocks"`):
			assignIfMissing("public_pages_v1", v)
		case strings.Contains(v, `"serverFilter"`) || strings.Contains(v, `"announcement"`):
			assignIfMissing("public_display_v1", v)
		}
	}

	_, _ = facades.Orm().Query().Table("system_settings").
		Where("setting_key = ? OR setting_key IS NULL", "").
		Delete()

	// 尝试补唯一索引；已存在时忽略错误
	_ = facades.Schema().Table("system_settings", func(table schema.Blueprint) {
		table.Unique("setting_key")
	})
	return nil
}

func (r *M20260723000001RepairPublicSettingsKeys) Down() error {
	_ = facades.Schema().Table("system_settings", func(table schema.Blueprint) {
		table.DropUnique("setting_key")
	})
	return nil
}
