package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000009CreateAlertsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000009CreateAlertsTable) Signature() string {
	return "20241207000009_create_alerts_table"
}

// Up Run the migrations.
func (r *M20241207000009CreateAlertsTable) Up() error {
	if !facades.Schema().HasTable("alerts") {
		return facades.Schema().Create("alerts", func(table schema.Blueprint) {
			table.String("id", 255)                   // 告警唯一标识符
			table.Primary("id")                       // 设置主键
			table.String("server_id", 255)            // 关联的服务器ID
			table.UnsignedBigInteger("rule_id")       // 关联的告警规则ID
			table.String("type", 20)                  // 告警类型
			table.String("title", 200)                // 告警标题
			table.Text("message").Nullable()          // 告警详细信息
			table.Decimal("metric_value").Nullable()  // 触发告警的指标值
			table.Boolean("is_read").Default(false)   // 是否已读
			table.Timestamp("timestamp").UseCurrent() // 告警触发时间

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
			table.Foreign("rule_id").References("id").On("alert_rules")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000009CreateAlertsTable) Down() error {
	return facades.Schema().DropIfExists("alerts")
}
