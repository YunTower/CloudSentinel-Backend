package seeders

import (
	"time"

	"github.com/goravel/framework/facades"
)

type MonitorConfigSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *MonitorConfigSeeder) Signature() string {
	return "MonitorConfigSeeder"
}

// Run executes the seeder logic.
func (s *MonitorConfigSeeder) Run() error {
	// 先清空表，避免重复插入
	facades.Orm().Query().Table("monitor_config").Delete()

	// 获取当前Unix时间戳
	now := time.Now().Unix()

	// 插入监控配置
	err := facades.Orm().Query().Table("monitor_config").Create(map[string]interface{}{
		"refresh_interval":        30,
		"chart_data_points":       100,
		"enable_real_time_update": true,
		"cpu_threshold":           80.0,
		"memory_threshold":        80.0,
		"disk_threshold":          80.0,
		"created_at":              now,
		"updated_at":              now,
	})
	return err
}
