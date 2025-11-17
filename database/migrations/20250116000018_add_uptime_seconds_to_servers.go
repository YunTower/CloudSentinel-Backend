package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000018AddUptimeSecondsToServers struct{}

// Signature The unique signature for the migration.
func (r *M20250116000018AddUptimeSecondsToServers) Signature() string {
	return "20250116000018_add_uptime_seconds_to_servers"
}

// Up Run the migrations.
func (r *M20250116000018AddUptimeSecondsToServers) Up() error {
	if facades.Schema().HasTable("servers") {
		return facades.Schema().Table("servers", func(table schema.Blueprint) {
			// 添加系统运行时间字段
			if !facades.Schema().HasColumn("servers", "uptime_seconds") {
				table.BigInteger("uptime_seconds").Default(0).Nullable() // 系统运行时间（秒）
			}
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000018AddUptimeSecondsToServers) Down() error {
	if facades.Schema().HasTable("servers") {
		return facades.Schema().Table("servers", func(table schema.Blueprint) {
			table.DropColumn("uptime_seconds")
		})
	}
	return nil
}
