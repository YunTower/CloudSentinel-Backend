package seeders

import (
	"github.com/goravel/framework/facades"
)

type AlertRulesSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *AlertRulesSeeder) Signature() string {
	return "AlertRulesSeeder"
}

// Run executes the seeder logic.
func (s *AlertRulesSeeder) Run() error {
	// 先清空表，避免重复插入
	facades.Orm().Query().Table("alert_rules").Delete()

	// 注意：新的 alert_rules 表结构已改为服务监控类型
	// 不再使用 metric_type, warning_threshold, critical_threshold
	// 而是使用 monitor_type, target, interval 等字段
	// 这里不再插入默认数据，因为服务监控规则需要用户手动配置

	return nil
}
