package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000010CreateServiceMonitorRuleServersTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000010CreateServiceMonitorRuleServersTable) Signature() string {
	return "20250116000010_create_service_monitor_rule_servers_table"
}

// Up Run the migrations.
func (r *M20250116000010CreateServiceMonitorRuleServersTable) Up() error {
	if !facades.Schema().HasTable("service_monitor_rule_servers") {
		return facades.Schema().Create("service_monitor_rule_servers", func(table schema.Blueprint) {
			table.ID()
			table.UnsignedBigInteger("rule_id")                         // 关联告警规则ID
			table.String("server_id", 255)                              // 关联服务器ID
			table.Timestamp("created_at").UseCurrent()                   // 创建时间
			table.Unique("rule_id", "server_id")                         // 唯一索引：rule_id + server_id
			table.Foreign("rule_id").References("id").On("alert_rules") // 外键关联规则
			table.Foreign("server_id").References("id").On("servers")    // 外键关联服务器
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000010CreateServiceMonitorRuleServersTable) Down() error {
	return facades.Schema().DropIfExists("service_monitor_rule_servers")
}

