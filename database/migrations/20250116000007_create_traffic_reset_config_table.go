package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000007CreateTrafficResetConfigTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000007CreateTrafficResetConfigTable) Signature() string {
	return "20250116000007_create_traffic_reset_config_table"
}

// Up Run the migrations.
func (r *M20250116000007CreateTrafficResetConfigTable) Up() error {
	if !facades.Schema().HasTable("traffic_reset_config") {
		return facades.Schema().Create("traffic_reset_config", func(table schema.Blueprint) {
			table.ID()
			table.Integer("reset_day").Default(1)    // 每月重置日期（1-28）
			table.Integer("reset_hour").Default(0)   // 重置时间（小时，0-23）
			table.Timestamps()                       // created_at, updated_at
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000007CreateTrafficResetConfigTable) Down() error {
	return facades.Schema().DropIfExists("traffic_reset_config")
}

