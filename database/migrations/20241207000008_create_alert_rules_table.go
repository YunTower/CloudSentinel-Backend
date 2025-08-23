package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000008CreateAlertRulesTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000008CreateAlertRulesTable) Signature() string {
	return "20241207000008_create_alert_rules_table"
}

// Up Run the migrations.
func (r *M20241207000008CreateAlertRulesTable) Up() error {
	if !facades.Schema().HasTable("alert_rules") {
		return facades.Schema().Create("alert_rules", func(table schema.Blueprint) {
			table.ID()
			table.String("rule_name", 100)         // 告警规则名称
			table.String("metric_type", 20)        // 监控指标类型
			table.Decimal("warning_threshold")     // 警告阈值（百分比）
			table.Decimal("critical_threshold")    // 严重阈值（百分比）
			table.Boolean("enabled").Default(true) // 是否启用该规则
			table.Timestamps()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000008CreateAlertRulesTable) Down() error {
	return facades.Schema().DropIfExists("alert_rules")
}
