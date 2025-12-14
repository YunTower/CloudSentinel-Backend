package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000005CreateAlertRulesTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000005CreateAlertRulesTable) Signature() string {
	return "20250129000005_create_alert_rules_table"
}

// Up Run the migrations.
func (r *M20250129000005CreateAlertRulesTable) Up() error {
	if !facades.Schema().HasTable("alert_rules") {
		return facades.Schema().Create("alert_rules", func(table schema.Blueprint) {
			table.ID()
			table.String("rule_name")
			table.String("monitor_type")
			table.String("target")
			table.Boolean("show_to_guest").Default(false)
			table.Integer("interval")
			table.Integer("notification_group_id").Nullable()
			table.Boolean("enable_failure_notification").Default(false)
			table.Boolean("enabled").Default(true)
			table.Timestamps()

			// 外键约束
			table.Foreign("notification_group_id").References("id").On("alert_notifications")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000005CreateAlertRulesTable) Down() error {
	return facades.Schema().DropIfExists("alert_rules")
}
