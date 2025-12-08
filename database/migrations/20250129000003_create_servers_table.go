package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000003CreateServersTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000003CreateServersTable) Signature() string {
	return "20250129000003_create_servers_table"
}

// Up Run the migrations.
func (r *M20250129000003CreateServersTable) Up() error {
	if !facades.Schema().HasTable("servers") {
		return facades.Schema().Create("servers", func(table schema.Blueprint) {
			table.String("id").NotNull()
			table.Primary("id")
			table.String("name").NotNull()
			table.String("ip").NotNull()
			table.Integer("port").Default(22).NotNull()
			table.String("status", 20).Default("offline").NotNull()
			table.String("location").Nullable()
			table.String("os").Nullable()
			table.String("architecture").Nullable()
			table.String("kernel").Nullable()
			table.String("hostname").Nullable()
			table.Integer("total_disks").Default(1).NotNull()
			table.Integer("cores").Default(1).NotNull()
			table.Timestamps()
			table.String("agent_version").Nullable()
			table.String("system_name").Nullable()
			table.Timestamp("boot_time").Nullable()
			table.Timestamp("last_report_time").Nullable()
			table.Integer("uptime_days").Default(0).NotNull()
			table.String("agent_key").Nullable()
			table.Integer("uptime_seconds").Default(0)
			table.Integer("group_id").Nullable()
			table.String("billing_cycle").Nullable()
			table.Integer("custom_cycle_days").Nullable()
			table.Decimal("price").Nullable()
			table.Timestamp("expire_time").Nullable()
			table.Integer("bandwidth_mbps").Default(0).NotNull()
			table.String("traffic_limit_type").Nullable()
			table.Integer("traffic_limit_bytes").Default(0).NotNull()
			table.String("traffic_reset_cycle").Nullable()
			table.Integer("traffic_custom_cycle_days").Nullable()
			table.Text("agent_public_key").Nullable()
			table.String("agent_fingerprint").Nullable()

			// 外键约束
			table.Foreign("group_id").References("id").On("server_groups")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000003CreateServersTable) Down() error {
	return facades.Schema().DropIfExists("servers")
}
