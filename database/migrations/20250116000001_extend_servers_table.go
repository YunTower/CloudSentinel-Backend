package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000001ExtendServersTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000001ExtendServersTable) Signature() string {
	return "20250116000001_extend_servers_table"
}

// Up Run the migrations.
func (r *M20250116000001ExtendServersTable) Up() error {
	if facades.Schema().HasTable("servers") {
		return facades.Schema().Table("servers", func(table schema.Blueprint) {
			// 扩展servers表，添加探针相关字段
			if !facades.Schema().HasColumn("servers", "agent_version") {
				table.String("agent_version", 50).Nullable() // 探针版本
			}
			if !facades.Schema().HasColumn("servers", "system_name") {
				table.String("system_name", 100).Nullable() // 系统名称
			}
			if !facades.Schema().HasColumn("servers", "boot_time") {
				table.Timestamp("boot_time").Nullable() // 启动时间
			}
			if !facades.Schema().HasColumn("servers", "last_report_time") {
				table.Timestamp("last_report_time").Nullable() // 最后上报时间
			}
			if !facades.Schema().HasColumn("servers", "uptime_days") {
				table.Integer("uptime_days").Default(0) // 运行天数
			}
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000001ExtendServersTable) Down() error {
	if facades.Schema().HasTable("servers") {
		return facades.Schema().Table("servers", func(table schema.Blueprint) {
			table.DropColumn("agent_version")
			table.DropColumn("system_name")
			table.DropColumn("boot_time")
			table.DropColumn("last_report_time")
			table.DropColumn("uptime_days")
		})
	}
	return nil
}

