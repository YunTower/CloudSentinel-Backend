package seeders

import (
	"goravel/app/repositories"
)

type TrafficResetConfigSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *TrafficResetConfigSeeder) Signature() string {
	return "TrafficResetConfigSeeder"
}

// Run executes the seeder logic.
func (s *TrafficResetConfigSeeder) Run() error {
	settingRepo := repositories.GetSystemSettingRepository()

	// 检查是否已存在配置
	_, err := settingRepo.GetByKey("traffic_reset_config")
	if err == nil {
		// 配置已存在，跳过
		return nil
	}

	// 插入流量重置配置（默认每月1日0点重置）
	trafficData := map[string]interface{}{
		"reset_day":  1,
		"reset_hour": 0,
	}

	return settingRepo.SetJSON("traffic_reset_config", trafficData)
}
