package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000019CreateServiceMonitorAlertsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000019CreateServiceMonitorAlertsTable) Signature() string {
	return "20250129000019_create_service_monitor_alerts_table"
}

// Up Run the migrations.
func (r *M20250129000019CreateServiceMonitorAlertsTable) Up() error {
	if !facades.Schema().HasTable("service_monitor_alerts") {
		return facades.Schema().Create("service_monitor_alerts", func(table schema.Blueprint) {
			table.String("id").NotNull()
			table.Primary("id")
			table.Integer("rule_id").NotNull()
			table.String("server_id").NotNull()
			table.String("type").NotNull()
			table.String("title").NotNull()
			table.Text("message").Nullable()
			table.Integer("response_time").Nullable()
			table.Boolean("is_read").Default(false).NotNull()
			table.Timestamp("timestamp").UseCurrent().NotNull()

			// 外键约束
			table.Foreign("rule_id").References("id").On("alert_rules")
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000019CreateServiceMonitorAlertsTable) Down() error {
	return facades.Schema().DropIfExists("service_monitor_alerts")
}
