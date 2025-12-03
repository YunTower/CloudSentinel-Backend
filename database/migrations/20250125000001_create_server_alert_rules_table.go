package migrations

import (
	"encoding/json"
	"goravel/app/repositories"

	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250125000001CreateServerAlertRulesTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250125000001CreateServerAlertRulesTable) Signature() string {
	return "20250125000001_create_server_alert_rules_table"
}

// Up Run the migrations.
func (r *M20250125000001CreateServerAlertRulesTable) Up() error {
	// 创建 server_alert_rules 表
	if !facades.Schema().HasTable("server_alert_rules") {
		err := facades.Schema().Create("server_alert_rules", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 36).Nullable() // NULL 表示全局规则
			table.String("rule_type", 50)            // cpu, memory, disk, bandwidth, traffic, expiration
			table.Text("config")                      // JSON 格式配置
			table.Timestamps()
		})
		if err != nil {
			return err
		}

		// 创建索引
		facades.Schema().Table("server_alert_rules", func(table schema.Blueprint) {
			table.Index("server_id")
			table.Index("rule_type")
			table.Unique("server_id", "rule_type")
		})
	}

	// 从 system_settings 迁移告警规则到新表
	settingRepo := repositories.GetSystemSettingRepository()
	ruleTypes := []string{"cpu", "memory", "disk"}

	for _, ruleType := range ruleTypes {
		key := "alert_rule_" + ruleType
		setting, err := settingRepo.GetByKey(key)
		if err != nil || setting == nil {
			// 如果不存在，创建默认规则
			defaultRule := map[string]interface{}{
				"enabled":  true,
				"warning":  80.0,
				"critical": 90.0,
			}
			if ruleType == "memory" || ruleType == "disk" {
				defaultRule["warning"] = 85.0
				defaultRule["critical"] = 95.0
			}
			configJson, _ := json.Marshal(defaultRule)
			_, err = facades.Orm().Query().Exec(
				"INSERT INTO server_alert_rules (server_id, rule_type, config, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))",
				nil, ruleType, string(configJson))
			if err != nil {
				facades.Log().Warningf("创建默认告警规则失败 %s: %v", ruleType, err)
			}
			continue
		}

		// 解析现有规则配置
		ruleValue := setting.GetValue()
		if ruleValue == "" {
			// 如果值为空，创建默认规则
			defaultRule := map[string]interface{}{
				"enabled":  true,
				"warning":  80.0,
				"critical": 90.0,
			}
			if ruleType == "memory" || ruleType == "disk" {
				defaultRule["warning"] = 85.0
				defaultRule["critical"] = 95.0
			}
			configJson, _ := json.Marshal(defaultRule)
			_, err = facades.Orm().Query().Exec(
				"INSERT INTO server_alert_rules (server_id, rule_type, config, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))",
				nil, ruleType, string(configJson))
			if err != nil {
				facades.Log().Warningf("创建默认告警规则失败 %s: %v", ruleType, err)
			}
			continue
		}

		// 验证 JSON 格式
		var ruleConfig map[string]interface{}
		if err := json.Unmarshal([]byte(ruleValue), &ruleConfig); err != nil {
			facades.Log().Warningf("解析告警规则配置失败 %s: %v", ruleType, err)
			// 使用默认规则
			defaultRule := map[string]interface{}{
				"enabled":  true,
				"warning":  80.0,
				"critical": 90.0,
			}
			if ruleType == "memory" || ruleType == "disk" {
				defaultRule["warning"] = 85.0
				defaultRule["critical"] = 95.0
			}
			configJson, _ := json.Marshal(defaultRule)
			_, err = facades.Orm().Query().Exec(
				"INSERT INTO server_alert_rules (server_id, rule_type, config, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))",
				nil, ruleType, string(configJson))
			if err != nil {
				facades.Log().Warningf("创建默认告警规则失败 %s: %v", ruleType, err)
			}
			continue
		}

		// 插入到新表（server_id 为 NULL 表示全局规则）
		_, err = facades.Orm().Query().Exec(
			"INSERT INTO server_alert_rules (server_id, rule_type, config, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))",
			nil, ruleType, ruleValue)
		if err != nil {
			facades.Log().Warningf("迁移告警规则失败 %s: %v", ruleType, err)
		}
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250125000001CreateServerAlertRulesTable) Down() error {
	// 回滚时删除表
	if facades.Schema().HasTable("server_alert_rules") {
		return facades.Schema().DropIfExists("server_alert_rules")
	}
	return nil
}

