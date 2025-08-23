package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000006CreateServerMetricsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000006CreateServerMetricsTable) Signature() string {
	return "20241207000006_create_server_metrics_table"
}

// Up Run the migrations.
func (r *M20241207000006CreateServerMetricsTable) Up() error {
	if !facades.Schema().HasTable("server_metrics") {
		return facades.Schema().Create("server_metrics", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255) // 关联的服务器ID
			table.Decimal("cpu_usage") // CPU使用率（百分比）
			table.Decimal("memory_usage") // 内存使用率（百分比）
			table.Decimal("disk_usage") // 磁盘使用率（百分比）
			table.Decimal("network_upload").Default(0) // 网络上传速度（KB/s）
			table.Decimal("network_download").Default(0) // 网络下载速度（KB/s）
			table.String("uptime", 100).Nullable() // 系统运行时间
			table.Timestamp("timestamp").UseCurrent() // 数据采集时间
			
			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000006CreateServerMetricsTable) Down() error {
	return facades.Schema().DropIfExists("server_metrics")
}
