package seeders

import (
	"goravel/app/repositories"
)

type MonitorConfigSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *MonitorConfigSeeder) Signature() string {
	return "MonitorConfigSeeder"
}

// Run executes the seeder logic.
func (s *MonitorConfigSeeder) Run() error {
	settingRepo := repositories.GetSystemSettingRepository()

	// 检查是否已存在配置
	_, err := settingRepo.GetByKey("monitor_config")
	if err == nil {
		// 配置已存在，跳过
		return nil
	}

	// 插入监控配置（JSON格式）
	monitorData := map[string]interface{}{
		"refresh_interval":        30,
		"chart_data_points":       100,
		"enable_real_time_update": true,
		"cpu_threshold":           80.0,
		"memory_threshold":        80.0,
		"disk_threshold":          80.0,
	}

	return settingRepo.SetJSON("monitor_config", monitorData)
}
