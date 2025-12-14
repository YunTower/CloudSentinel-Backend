package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000008CreateAlertsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000008CreateAlertsTable) Signature() string {
	return "20250129000008_create_alerts_table"
}

// Up Run the migrations.
func (r *M20250129000008CreateAlertsTable) Up() error {
	if !facades.Schema().HasTable("alerts") {
		return facades.Schema().Create("alerts", func(table schema.Blueprint) {
			table.String("id")
			table.Primary("id")
			table.String("server_id")
			table.Integer("rule_id")
			table.String("type")
			table.String("title")
			table.Text("message").Nullable()
			table.Decimal("metric_value").Nullable()
			table.Boolean("is_read").Default(false)
			table.Timestamp("timestamp").UseCurrent()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
			table.Foreign("rule_id").References("id").On("alert_rules")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000008CreateAlertsTable) Down() error {
	return facades.Schema().DropIfExists("alerts")
}
