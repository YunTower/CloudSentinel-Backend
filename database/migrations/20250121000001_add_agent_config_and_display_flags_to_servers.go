package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250121000001AddAgentConfigAndDisplayFlagsToServers struct{}

// Signature The unique signature for the migration.
func (r *M20250121000001AddAgentConfigAndDisplayFlagsToServers) Signature() string {
	return "20250121000001_add_agent_config_and_display_flags_to_servers"
}

// Up Run the migrations.
func (r *M20250121000001AddAgentConfigAndDisplayFlagsToServers) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		// Agent配置字段
		table.String("agent_timezone", 50).Default("Asia/Shanghai")
		table.Integer("agent_metrics_interval").Default(5)
		table.Integer("agent_detail_interval").Default(15)
		table.Integer("agent_system_interval").Default(15)
		table.Integer("agent_heartbeat_interval").Default(10)
		table.String("agent_log_path", 255).Default("logs")

		// 显示开关字段
		table.Boolean("show_billing_cycle").Default(false)
		table.Boolean("show_traffic_limit").Default(false)
		table.Boolean("show_traffic_reset_cycle").Default(false)
	})
}

// Down Reverse the migrations.
func (r *M20250121000001AddAgentConfigAndDisplayFlagsToServers) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn(
			"agent_timezone",
			"agent_metrics_interval",
			"agent_detail_interval",
			"agent_system_interval",
			"agent_heartbeat_interval",
			"agent_log_path",
			"show_billing_cycle",
			"show_traffic_limit",
			"show_traffic_reset_cycle",
		)
	})
}
