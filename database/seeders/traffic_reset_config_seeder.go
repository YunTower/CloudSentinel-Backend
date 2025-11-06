package seeders

import (
	"time"

	"github.com/goravel/framework/facades"
)

type TrafficResetConfigSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *TrafficResetConfigSeeder) Signature() string {
	return "TrafficResetConfigSeeder"
}

// Run executes the seeder logic.
func (s *TrafficResetConfigSeeder) Run() error {
	// 先清空表，避免重复插入
	facades.Orm().Query().Table("traffic_reset_config").Delete()

	// 获取当前Unix时间戳
	now := time.Now().Unix()

	// 插入流量重置配置（默认每月1日0点重置）
	err := facades.Orm().Query().Table("traffic_reset_config").Create(map[string]interface{}{
		"reset_day":  1,
		"reset_hour": 0,
		"created_at": now,
		"updated_at": now,
	})

	return err
}

