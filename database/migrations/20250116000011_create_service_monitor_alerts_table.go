package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000011CreateServiceMonitorAlertsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000011CreateServiceMonitorAlertsTable) Signature() string {
	return "20250116000011_create_service_monitor_alerts_table"
}

// Up Run the migrations.
func (r *M20250116000011CreateServiceMonitorAlertsTable) Up() error {
	if !facades.Schema().HasTable("service_monitor_alerts") {
		return facades.Schema().Create("service_monitor_alerts", func(table schema.Blueprint) {
			table.String("id", 255)                                   // 告警唯一标识符
			table.Primary("id")                                        // 设置主键
			table.UnsignedBigInteger("rule_id")                       // 关联规则ID
			table.String("server_id", 255)                             // 关联服务器ID
			table.String("type", 20)                                  // 告警类型：'error', 'warning', 'info'
			table.String("title", 200)                                 // 告警标题
			table.Text("message").Nullable()                           // 告警详情
			table.Integer("response_time").Nullable()                   // 响应时间（毫秒，用于HTTP/TCP）
			table.Boolean("is_read").Default(false)                     // 是否已读
			table.Timestamp("timestamp").UseCurrent()                  // 告警时间
			table.Foreign("rule_id").References("id").On("alert_rules") // 外键关联规则
			table.Foreign("server_id").References("id").On("servers")    // 外键关联服务器
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000011CreateServiceMonitorAlertsTable) Down() error {
	return facades.Schema().DropIfExists("service_monitor_alerts")
}

