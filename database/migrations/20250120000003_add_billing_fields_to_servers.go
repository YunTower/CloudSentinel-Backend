package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250120000003AddBillingFieldsToServers struct {
}

// Signature The unique signature for the migration.
func (r *M20250120000003AddBillingFieldsToServers) Signature() string {
	return "20250120000003_add_billing_fields_to_servers"
}

// Up Run the migrations.
func (r *M20250120000003AddBillingFieldsToServers) Up() error {
	if facades.Schema().HasTable("servers") {
		return facades.Schema().Table("servers", func(table schema.Blueprint) {
			// 分组关联
			table.Integer("group_id").Nullable() // 关联 server_groups.id，可空
			// 付费周期相关
			table.String("billing_cycle", 20).Nullable()  // 付费周期：'monthly', 'quarterly', 'yearly', 'one_time', 'custom'
			table.Integer("custom_cycle_days").Nullable() // 自定义周期天数（仅当 billing_cycle='custom' 时使用）
			table.Decimal("price").Nullable()             // 价格
			table.Timestamp("expire_time").Nullable()     // 到期时间，可空
			// 带宽和流量限制
			table.Integer("bandwidth_mbps").Default(0)            // 带宽大小（Mbps），0表示无限制
			table.String("traffic_limit_type", 20).Nullable()     // 流量限制类型：'unlimited', 'permanent', 'periodic'
			table.BigInteger("traffic_limit_bytes").Default(0)    // 流量限制大小（字节），0表示无限制
			table.String("traffic_reset_cycle", 20).Nullable()    // 流量重置周期：'monthly', 'quarterly', 'yearly', 'custom'
			table.Integer("traffic_custom_cycle_days").Nullable() // 自定义流量重置周期天数
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250120000003AddBillingFieldsToServers) Down() error {
	if facades.Schema().HasTable("servers") {
		return facades.Schema().Table("servers", func(table schema.Blueprint) {
			table.DropColumn("group_id")
			table.DropColumn("billing_cycle")
			table.DropColumn("custom_cycle_days")
			table.DropColumn("price")
			table.DropColumn("expire_time")
			table.DropColumn("bandwidth_mbps")
			table.DropColumn("traffic_limit_type")
			table.DropColumn("traffic_limit_bytes")
			table.DropColumn("traffic_reset_cycle")
			table.DropColumn("traffic_custom_cycle_days")
		})
	}

	return nil
}
