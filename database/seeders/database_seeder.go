package seeders

import (
	"github.com/goravel/framework/contracts/database/seeder"
	"github.com/goravel/framework/facades"
)

type DatabaseSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *DatabaseSeeder) Signature() string {
	return "DatabaseSeeder"
}

// Run executes the seeder logic.
func (s *DatabaseSeeder) Run() error {
	return facades.Seeder().Call([]seeder.Seeder{
		&SystemSettingsSeeder{},
		&MonitorConfigSeeder{},
		&TrafficResetConfigSeeder{},
		&LogCleanupConfigSeeder{},
		&AlertRulesSeeder{}, // 注意：新的 alert_rules 表不再插入默认数据
	})
}
