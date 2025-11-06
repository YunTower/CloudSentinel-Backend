package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000012CreateLogCleanupConfigTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000012CreateLogCleanupConfigTable) Signature() string {
	return "20250116000012_create_log_cleanup_config_table"
}

// Up Run the migrations.
func (r *M20250116000012CreateLogCleanupConfigTable) Up() error {
	if !facades.Schema().HasTable("log_cleanup_config") {
		return facades.Schema().Create("log_cleanup_config", func(table schema.Blueprint) {
			table.ID()
			table.String("log_type", 50)                               // 日志类型：'server_metrics', 'alerts', 'audit_logs'等
			table.Integer("cleanup_interval_days")                      // 清理间隔（天数）
			table.Integer("keep_days")                                   // 保留天数
			table.Boolean("enabled").Default(true)                      // 是否启用
			table.Timestamp("last_cleanup_time").Nullable()              // 上次清理时间
			table.Timestamps()                                           // created_at, updated_at
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000012CreateLogCleanupConfigTable) Down() error {
	return facades.Schema().DropIfExists("log_cleanup_config")
}

