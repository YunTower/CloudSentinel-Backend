package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000006CreateServerAlertRulesTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000006CreateServerAlertRulesTable) Signature() string {
	return "20250129000006_create_server_alert_rules_table"
}

// Up Run the migrations.
func (r *M20250129000006CreateServerAlertRulesTable) Up() error {
	if !facades.Schema().HasTable("server_alert_rules") {
		err := facades.Schema().Create("server_alert_rules", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id").Nullable()
			table.String("rule_type")
			table.Text("config")
			table.Timestamps()
		})
		if err != nil {
			return err
		}

		// 创建索引
		facades.Schema().Table("server_alert_rules", func(table schema.Blueprint) {
			table.Index("server_alert_rules_server_id_index", "server_id")
			table.Index("server_alert_rules_rule_type_index", "rule_type")
			table.Unique("server_alert_rules_server_id_rule_type_unique", "server_id", "rule_type")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000006CreateServerAlertRulesTable) Down() error {
	return facades.Schema().DropIfExists("server_alert_rules")
}
