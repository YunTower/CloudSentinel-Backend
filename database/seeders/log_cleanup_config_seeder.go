package seeders

import (
	"goravel/app/repositories"
)

type LogCleanupConfigSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *LogCleanupConfigSeeder) Signature() string {
	return "LogCleanupConfigSeeder"
}

// Run executes the seeder logic.
func (s *LogCleanupConfigSeeder) Run() error {
	settingRepo := repositories.GetSystemSettingRepository()

	// 检查是否已存在配置
	_, err := settingRepo.GetByKey("log_cleanup_config")
	if err == nil {
		// 配置已存在，跳过
		return nil
	}

	// 插入默认日志清理配置（数组格式）
	cleanupConfigs := []map[string]interface{}{
		{
			"log_type":              "server_metrics",
			"cleanup_interval_days": 7,
			"keep_days":             30,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "server_memory_history",
			"cleanup_interval_days": 7,
			"keep_days":             30,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "server_swap",
			"cleanup_interval_days": 7,
			"keep_days":             30,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "server_network_connections",
			"cleanup_interval_days": 7,
			"keep_days":             30,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "server_network_speed",
			"cleanup_interval_days": 7,
			"keep_days":             30,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "server_cpus",
			"cleanup_interval_days": 7,
			"keep_days":             30,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "alerts",
			"cleanup_interval_days": 7,
			"keep_days":             90,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "service_monitor_alerts",
			"cleanup_interval_days": 7,
			"keep_days":             90,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
		{
			"log_type":              "audit_logs",
			"cleanup_interval_days": 7,
			"keep_days":             180,
			"enabled":               true,
			"last_cleanup_time":     nil,
		},
	}

	return settingRepo.SetJSON("log_cleanup_config", cleanupConfigs)
}
