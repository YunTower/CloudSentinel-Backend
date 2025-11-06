package seeders

import (
	"time"

	"github.com/goravel/framework/facades"
)

type LogCleanupConfigSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *LogCleanupConfigSeeder) Signature() string {
	return "LogCleanupConfigSeeder"
}

// Run executes the seeder logic.
func (s *LogCleanupConfigSeeder) Run() error {
	// 先清空表，避免重复插入
	facades.Orm().Query().Table("log_cleanup_config").Delete()

	// 获取当前Unix时间戳
	now := time.Now().Unix()

	// 插入默认日志清理配置
	cleanupConfigs := []map[string]interface{}{
		{
			"log_type":             "server_metrics",
			"cleanup_interval_days": 7,
			"keep_days":            30,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "server_memory_history",
			"cleanup_interval_days": 7,
			"keep_days":            30,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "server_virtual_memory",
			"cleanup_interval_days": 7,
			"keep_days":            30,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "server_network_connections",
			"cleanup_interval_days": 7,
			"keep_days":            30,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "server_network_speed",
			"cleanup_interval_days": 7,
			"keep_days":            30,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "server_cpus",
			"cleanup_interval_days": 7,
			"keep_days":            30,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "alerts",
			"cleanup_interval_days": 7,
			"keep_days":            90,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "service_monitor_alerts",
			"cleanup_interval_days": 7,
			"keep_days":            90,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
		{
			"log_type":             "audit_logs",
			"cleanup_interval_days": 7,
			"keep_days":            180,
			"enabled":              true,
			"last_cleanup_time":    nil,
			"created_at":           now,
			"updated_at":           now,
		},
	}

	for _, config := range cleanupConfigs {
		err := facades.Orm().Query().Table("log_cleanup_config").Create(config)
		if err != nil {
			return err
		}
	}

	return nil
}

