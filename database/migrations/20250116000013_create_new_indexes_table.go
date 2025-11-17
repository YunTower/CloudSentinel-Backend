package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000013CreateNewIndexesTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000013CreateNewIndexesTable) Signature() string {
	return "20250116000013_create_new_indexes_table"
}

// Up Run the migrations.
func (r *M20250116000013CreateNewIndexesTable) Up() error {
	// server_cpus索引
	if facades.Schema().HasTable("server_cpus") {
		facades.Schema().Table("server_cpus", func(table schema.Blueprint) {
			table.Index("idx_server_cpus_server_id", "server_id")
			table.Index("idx_server_cpus_timestamp", "timestamp")
		})
	}

	// server_memory_history索引
	if facades.Schema().HasTable("server_memory_history") {
		facades.Schema().Table("server_memory_history", func(table schema.Blueprint) {
			table.Index("idx_server_memory_history_server_id", "server_id")
			table.Index("idx_server_memory_history_timestamp", "timestamp")
		})
	}

	// server_swap索引
	if facades.Schema().HasTable("server_swap") {
		facades.Schema().Table("server_swap", func(table schema.Blueprint) {
			table.Index("idx_server_swap_server_id", "server_id")
			table.Index("idx_server_swap_timestamp", "timestamp")
		})
	}

	// server_network_connections索引
	if facades.Schema().HasTable("server_network_connections") {
		facades.Schema().Table("server_network_connections", func(table schema.Blueprint) {
			table.Index("idx_server_network_connections_server_id", "server_id")
			table.Index("idx_server_network_connections_timestamp", "timestamp")
		})
	}

	// server_traffic_usage索引（唯一索引已在创建表时添加）
	if facades.Schema().HasTable("server_traffic_usage") {
		facades.Schema().Table("server_traffic_usage", func(table schema.Blueprint) {
			table.Index("idx_server_traffic_usage_server_id", "server_id")
			table.Index("idx_server_traffic_usage_year_month", "year", "month")
		})
	}

	// server_network_speed索引
	if facades.Schema().HasTable("server_network_speed") {
		facades.Schema().Table("server_network_speed", func(table schema.Blueprint) {
			table.Index("idx_server_network_speed_server_id", "server_id")
			table.Index("idx_server_network_speed_timestamp", "timestamp")
		})
	}

	// service_monitor_rule_servers索引（唯一索引已在创建表时添加）
	if facades.Schema().HasTable("service_monitor_rule_servers") {
		facades.Schema().Table("service_monitor_rule_servers", func(table schema.Blueprint) {
			table.Index("idx_service_monitor_rule_servers_rule_id", "rule_id")
			table.Index("idx_service_monitor_rule_servers_server_id", "server_id")
		})
	}

	// service_monitor_alerts索引
	if facades.Schema().HasTable("service_monitor_alerts") {
		facades.Schema().Table("service_monitor_alerts", func(table schema.Blueprint) {
			table.Index("idx_service_monitor_alerts_rule_id", "rule_id")
			table.Index("idx_service_monitor_alerts_server_id", "server_id")
			table.Index("idx_service_monitor_alerts_timestamp", "timestamp")
			table.Index("idx_service_monitor_alerts_is_read", "is_read")
		})
	}

	// alert_rules索引
	if facades.Schema().HasTable("alert_rules") {
		facades.Schema().Table("alert_rules", func(table schema.Blueprint) {
			table.Index("idx_alert_rules_monitor_type", "monitor_type")
			table.Index("idx_alert_rules_enabled", "enabled")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000013CreateNewIndexesTable) Down() error {
	// 删除索引
	if facades.Schema().HasTable("server_cpus") {
		facades.Schema().Table("server_cpus", func(table schema.Blueprint) {
			table.DropIndex("idx_server_cpus_server_id")
			table.DropIndex("idx_server_cpus_timestamp")
		})
	}

	if facades.Schema().HasTable("server_memory_history") {
		facades.Schema().Table("server_memory_history", func(table schema.Blueprint) {
			table.DropIndex("idx_server_memory_history_server_id")
			table.DropIndex("idx_server_memory_history_timestamp")
		})
	}

	if facades.Schema().HasTable("server_swap") {
		facades.Schema().Table("server_swap", func(table schema.Blueprint) {
			table.DropIndex("idx_server_swap_server_id")
			table.DropIndex("idx_server_swap_timestamp")
		})
	}

	if facades.Schema().HasTable("server_network_connections") {
		facades.Schema().Table("server_network_connections", func(table schema.Blueprint) {
			table.DropIndex("idx_server_network_connections_server_id")
			table.DropIndex("idx_server_network_connections_timestamp")
		})
	}

	if facades.Schema().HasTable("server_traffic_usage") {
		facades.Schema().Table("server_traffic_usage", func(table schema.Blueprint) {
			table.DropIndex("idx_server_traffic_usage_server_id")
			table.DropIndex("idx_server_traffic_usage_year_month")
		})
	}

	if facades.Schema().HasTable("server_network_speed") {
		facades.Schema().Table("server_network_speed", func(table schema.Blueprint) {
			table.DropIndex("idx_server_network_speed_server_id")
			table.DropIndex("idx_server_network_speed_timestamp")
		})
	}

	if facades.Schema().HasTable("service_monitor_rule_servers") {
		facades.Schema().Table("service_monitor_rule_servers", func(table schema.Blueprint) {
			table.DropIndex("idx_service_monitor_rule_servers_rule_id")
			table.DropIndex("idx_service_monitor_rule_servers_server_id")
		})
	}

	if facades.Schema().HasTable("service_monitor_alerts") {
		facades.Schema().Table("service_monitor_alerts", func(table schema.Blueprint) {
			table.DropIndex("idx_service_monitor_alerts_rule_id")
			table.DropIndex("idx_service_monitor_alerts_server_id")
			table.DropIndex("idx_service_monitor_alerts_timestamp")
			table.DropIndex("idx_service_monitor_alerts_is_read")
		})
	}

	if facades.Schema().HasTable("alert_rules") {
		facades.Schema().Table("alert_rules", func(table schema.Blueprint) {
			table.DropIndex("idx_alert_rules_monitor_type")
			table.DropIndex("idx_alert_rules_enabled")
		})
	}

	return nil
}
