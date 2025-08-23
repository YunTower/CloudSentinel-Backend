package seeders

import (
	"time"

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

	// 获取当前Unix时间戳
	now := time.Now().Unix()

	// 插入默认告警规则
	alertRules := []map[string]interface{}{
		{
			"rule_name":          "CPU告警",
			"metric_type":        "cpu",
			"warning_threshold":  80.0,
			"critical_threshold": 95.0,
			"enabled":            true,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"rule_name":          "内存告警",
			"metric_type":        "memory",
			"warning_threshold":  80.0,
			"critical_threshold": 95.0,
			"enabled":            true,
			"created_at":         now,
			"updated_at":         now,
		},
		{
			"rule_name":          "磁盘告警",
			"metric_type":        "disk",
			"warning_threshold":  80.0,
			"critical_threshold": 95.0,
			"enabled":            true,
			"created_at":         now,
			"updated_at":         now,
		},
	}

	for _, rule := range alertRules {
		err := facades.Orm().Query().Table("alert_rules").Create(rule)
		if err != nil {
			return err
		}
	}

	return nil
}
