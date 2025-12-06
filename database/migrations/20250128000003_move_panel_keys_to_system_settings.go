package migrations

import (
	"encoding/json"

	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250128000003MovePanelKeysToSystemSettings struct{}

// Signature The unique signature for the migration.
func (m *M20250128000003MovePanelKeysToSystemSettings) Signature() string {
	return "20250128000003_move_panel_keys_to_system_settings"
}

// Up Run the migrations.
func (m *M20250128000003MovePanelKeysToSystemSettings) Up() error {
	// 从 servers 表中获取第一个有效的 panel 密钥对
	var servers []map[string]interface{}
	err := facades.Orm().Query().Table("servers").
		Select("panel_private_key", "panel_public_key").
		Where("panel_private_key", "!=", "").
		Where("panel_public_key", "!=", "").
		Limit(1).
		Get(&servers)

	var panelPrivateKey, panelPublicKey string
	if err == nil && len(servers) > 0 {
		if pk, ok := servers[0]["panel_private_key"].(string); ok && pk != "" {
			panelPrivateKey = pk
		}
		if pk, ok := servers[0]["panel_public_key"].(string); ok && pk != "" {
			panelPublicKey = pk
		}
	}

	// 将 panel 密钥对保存到 system_settings
	panelKeys := map[string]interface{}{
		"panel_private_key": panelPrivateKey,
		"panel_public_key":  panelPublicKey,
	}
	panelKeysJSON, _ := json.Marshal(panelKeys)

	// 检查是否已存在
	var existing []map[string]interface{}
	_ = facades.Orm().Query().Table("system_settings").
		Where("setting_key", "panel_rsa_keys").
		Get(&existing)

	if len(existing) > 0 {
		// 更新
		_, _ = facades.Orm().Query().Table("system_settings").
			Where("setting_key", "panel_rsa_keys").
			Update(map[string]interface{}{
				"setting_value": string(panelKeysJSON),
				"setting_type":  "json",
			})
	} else {
		// 插入（使用 Exec 执行 SQL）
		_, _ = facades.Orm().Query().Exec(
			"INSERT INTO system_settings (setting_key, setting_value, setting_type, description) VALUES (?, ?, ?, ?)",
			"panel_rsa_keys",
			string(panelKeysJSON),
			"json",
			"Panel RSA 密钥对（全局配置）",
		)
	}

	// 从 servers 表移除 panel 密钥字段
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("panel_private_key")
		table.DropColumn("panel_public_key")
	})
}

// Down Reverse the migrations.
func (m *M20250128000003MovePanelKeysToSystemSettings) Down() error {
	// 重新添加 panel 密钥字段到 servers 表
	err := facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.Text("panel_private_key").Nullable()
		table.Text("panel_public_key").Nullable()
	})
	if err != nil {
		return err
	}

	// 从 system_settings 读取 panel 密钥对并迁移回 servers 表
	var settings []map[string]interface{}
	err = facades.Orm().Query().Table("system_settings").
		Where("setting_key", "panel_rsa_keys").
		Get(&settings)

	if err == nil && len(settings) > 0 {
		var panelKeys map[string]interface{}
		if settingValue, ok := settings[0]["setting_value"].(string); ok {
			_ = json.Unmarshal([]byte(settingValue), &panelKeys)

			if panelPrivateKey, ok := panelKeys["panel_private_key"].(string); ok && panelPrivateKey != "" {
				if panelPublicKey, ok := panelKeys["panel_public_key"].(string); ok && panelPublicKey != "" {
					// 更新所有服务器（使用相同的 panel 密钥对）
					_, _ = facades.Orm().Query().Table("servers").
						Update(map[string]interface{}{
							"panel_private_key": panelPrivateKey,
							"panel_public_key":  panelPublicKey,
						})
				}
			}
		}
	}

	// 从 system_settings 删除 panel_rsa_keys
	_, _ = facades.Orm().Query().Table("system_settings").
		Where("setting_key", "panel_rsa_keys").
		Delete()

	return nil
}
