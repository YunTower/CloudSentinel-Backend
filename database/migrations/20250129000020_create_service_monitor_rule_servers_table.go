package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000020CreateServiceMonitorRuleServersTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000020CreateServiceMonitorRuleServersTable) Signature() string {
	return "20250129000020_create_service_monitor_rule_servers_table"
}

// Up Run the migrations.
func (r *M20250129000020CreateServiceMonitorRuleServersTable) Up() error {
	if !facades.Schema().HasTable("service_monitor_rule_servers") {
		err := facades.Schema().Create("service_monitor_rule_servers", func(table schema.Blueprint) {
			table.ID()
			table.Integer("rule_id")
			table.String("server_id")
			table.Timestamp("created_at").UseCurrent()

			// 外键约束
			table.Foreign("rule_id").References("id").On("alert_rules")
			table.Foreign("server_id").References("id").On("servers")
		})
		if err != nil {
			return err
		}

		// 创建唯一索引
		facades.Schema().Table("service_monitor_rule_servers", func(table schema.Blueprint) {
			table.Unique("rule_id", "server_id")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000020CreateServiceMonitorRuleServersTable) Down() error {
	return facades.Schema().DropIfExists("service_monitor_rule_servers")
}
