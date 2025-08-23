package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000011CreateMonitorConfigTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000011CreateMonitorConfigTable) Signature() string {
	return "20241207000011_create_monitor_config_table"
}

// Up Run the migrations.
func (r *M20241207000011CreateMonitorConfigTable) Up() error {
	if !facades.Schema().HasTable("monitor_config") {
		return facades.Schema().Create("monitor_config", func(table schema.Blueprint) {
			table.ID()
			table.Integer("refresh_interval").Default(30)          // 数据刷新间隔（秒）
			table.Integer("chart_data_points").Default(100)        // 图表显示的数据点数量
			table.Boolean("enable_real_time_update").Default(true) // 是否启用实时更新
			table.Decimal("cpu_threshold").Default(80)             // CPU告警阈值（百分比）
			table.Decimal("memory_threshold").Default(80)          // 内存告警阈值（百分比）
			table.Decimal("disk_threshold").Default(80)            // 磁盘告警阈值（百分比）
			table.Timestamps()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000011CreateMonitorConfigTable) Down() error {
	return facades.Schema().DropIfExists("monitor_config")
}
