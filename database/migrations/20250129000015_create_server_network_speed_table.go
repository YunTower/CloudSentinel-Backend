package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000015CreateServerNetworkSpeedTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000015CreateServerNetworkSpeedTable) Signature() string {
	return "20250129000015_create_server_network_speed_table"
}

// Up Run the migrations.
func (r *M20250129000015CreateServerNetworkSpeedTable) Up() error {
	if !facades.Schema().HasTable("server_network_speed") {
		return facades.Schema().Create("server_network_speed", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id")
			table.Decimal("upload_speed")
			table.Decimal("download_speed")
			table.Timestamp("timestamp").UseCurrent()
			table.Timestamps()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000015CreateServerNetworkSpeedTable) Down() error {
	return facades.Schema().DropIfExists("server_network_speed")
}
