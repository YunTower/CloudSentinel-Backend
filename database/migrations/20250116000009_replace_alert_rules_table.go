package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000009ReplaceAlertRulesTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000009ReplaceAlertRulesTable) Signature() string {
	return "20250116000009_replace_alert_rules_table"
}

// Up Run the migrations.
func (r *M20250116000009ReplaceAlertRulesTable) Up() error {
	// 删除旧表（如果存在）
	if facades.Schema().HasTable("alert_rules") {
		// 先删除外键约束
		if facades.Schema().HasTable("alerts") {
			facades.Schema().Table("alerts", func(table schema.Blueprint) {
				table.DropForeign("alerts_rule_id_foreign")
			})
		}
		// 删除旧表
		facades.Schema().DropIfExists("alert_rules")
	}

	// 创建新的服务监控告警规则表
	if !facades.Schema().HasTable("alert_rules") {
		return facades.Schema().Create("alert_rules", func(table schema.Blueprint) {
			table.ID()
			table.String("rule_name", 100)                              // 规则名称（自定义）
			table.String("monitor_type", 20)                            // 监控类型：'http_get', 'icmp_ping', 'tcping'
			table.String("target", 500)                                 // 监控目标（URL/IP:Port）
			table.Boolean("show_to_guest").Default(false)                // 是否向游客显示
			table.Integer("interval")                                   // 监控间隔（秒）
			table.UnsignedBigInteger("notification_group_id").Nullable() // 通知组ID（关联alert_notifications）
			table.Boolean("enable_failure_notification").Default(false) // 启用失败通知
			table.Boolean("enabled").Default(true)                      // 是否启用
			table.Timestamps()                                           // created_at, updated_at
			
			// 外键关联通知组（可选）
			if facades.Schema().HasTable("alert_notifications") {
				table.Foreign("notification_group_id").References("id").On("alert_notifications")
			}
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000009ReplaceAlertRulesTable) Down() error {
	// 恢复旧表结构（如果需要回滚）
	if facades.Schema().HasTable("alert_rules") {
		facades.Schema().DropIfExists("alert_rules")
		
		// 创建旧表结构
		return facades.Schema().Create("alert_rules", func(table schema.Blueprint) {
			table.ID()
			table.String("rule_name", 100)
			table.String("metric_type", 20)
			table.Decimal("warning_threshold")
			table.Decimal("critical_threshold")
			table.Boolean("enabled").Default(true)
			table.Timestamps()
		})
	}
	return nil
}

