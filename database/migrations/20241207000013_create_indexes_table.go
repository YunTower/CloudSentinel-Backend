package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000013CreateIndexesTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000013CreateIndexesTable) Signature() string {
	return "20241207000013_create_indexes_table"
}

// Up Run the migrations.
func (r *M20241207000013CreateIndexesTable) Up() error {
	// 服务器相关索引
	facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.Index("idx_servers_status", "status")
		table.Index("idx_servers_location", "location")
		table.Index("idx_servers_ip", "ip")
	})

	facades.Schema().Table("server_metrics", func(table schema.Blueprint) {
		table.Index("idx_server_metrics_server_id", "server_id")
		table.Index("idx_server_metrics_timestamp", "timestamp")
	})

	facades.Schema().Table("server_disks", func(table schema.Blueprint) {
		table.Index("idx_server_disks_server_id", "server_id")
		table.Index("idx_server_disks_mount_point", "mount_point")
		table.Index("idx_server_disks_type", "disk_type")
	})

	facades.Schema().Table("server_status_logs", func(table schema.Blueprint) {
		table.Index("idx_server_status_logs_server_id", "server_id")
		table.Index("idx_server_status_logs_timestamp", "timestamp")
	})

	// 告警相关索引
	facades.Schema().Table("alerts", func(table schema.Blueprint) {
		table.Index("idx_alerts_server_id", "server_id")
		table.Index("idx_alerts_type", "type")
		table.Index("idx_alerts_timestamp", "timestamp")
		table.Index("idx_alerts_is_read", "is_read")
	})

	facades.Schema().Table("alert_rules", func(table schema.Blueprint) {
		table.Index("idx_alert_rules_metric_type", "metric_type")
		table.Index("idx_alert_rules_enabled", "enabled")
	})

	// 系统配置索引
	facades.Schema().Table("system_settings", func(table schema.Blueprint) {
		table.Index("idx_system_settings_key", "setting_key")
	})

	// 审计日志索引
	facades.Schema().Table("audit_logs", func(table schema.Blueprint) {
		table.Index("idx_audit_logs_action", "action")
		table.Index("idx_audit_logs_timestamp", "timestamp")
		table.Index("idx_audit_logs_resource", "resource_type", "resource_id")
	})

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000013CreateIndexesTable) Down() error {
	// 删除索引
	facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropIndex("idx_servers_status")
		table.DropIndex("idx_servers_location")
		table.DropIndex("idx_servers_ip")
	})

	facades.Schema().Table("server_metrics", func(table schema.Blueprint) {
		table.DropIndex("idx_server_metrics_server_id")
		table.DropIndex("idx_server_metrics_timestamp")
	})

	facades.Schema().Table("server_disks", func(table schema.Blueprint) {
		table.DropIndex("idx_server_disks_server_id")
		table.DropIndex("idx_server_disks_mount_point")
		table.DropIndex("idx_server_disks_type")
	})

	facades.Schema().Table("server_status_logs", func(table schema.Blueprint) {
		table.DropIndex("idx_server_status_logs_server_id")
		table.DropIndex("idx_server_status_logs_timestamp")
	})

	facades.Schema().Table("alerts", func(table schema.Blueprint) {
		table.DropIndex("idx_alerts_server_id")
		table.DropIndex("idx_alerts_type")
		table.DropIndex("idx_alerts_timestamp")
		table.DropIndex("idx_alerts_is_read")
	})

	facades.Schema().Table("alert_rules", func(table schema.Blueprint) {
		table.DropIndex("idx_alert_rules_metric_type")
		table.DropIndex("idx_alert_rules_enabled")
	})

	facades.Schema().Table("system_settings", func(table schema.Blueprint) {
		table.DropIndex("idx_system_settings_key")
	})

	facades.Schema().Table("audit_logs", func(table schema.Blueprint) {
		table.DropIndex("idx_audit_logs_action")
		table.DropIndex("idx_audit_logs_timestamp")
		table.DropIndex("idx_audit_logs_resource")
	})

	return nil
}
