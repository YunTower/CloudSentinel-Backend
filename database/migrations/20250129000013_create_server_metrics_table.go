package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000013CreateServerMetricsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000013CreateServerMetricsTable) Signature() string {
	return "20250129000013_create_server_metrics_table"
}

// Up Run the migrations.
func (r *M20250129000013CreateServerMetricsTable) Up() error {
	if !facades.Schema().HasTable("server_metrics") {
		return facades.Schema().Create("server_metrics", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id")
			table.Decimal("cpu_usage")
			table.Decimal("memory_usage")
			table.Decimal("disk_usage")
			table.Decimal("network_upload").Default(0)
			table.Decimal("network_download").Default(0)
			table.String("uptime").Nullable()
			table.Timestamp("timestamp").UseCurrent()
			table.Timestamps()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000013CreateServerMetricsTable) Down() error {
	return facades.Schema().DropIfExists("server_metrics")
}
