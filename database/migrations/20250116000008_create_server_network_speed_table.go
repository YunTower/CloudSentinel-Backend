package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000008CreateServerNetworkSpeedTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000008CreateServerNetworkSpeedTable) Signature() string {
	return "20250116000008_create_server_network_speed_table"
}

// Up Run the migrations.
func (r *M20250116000008CreateServerNetworkSpeedTable) Up() error {
	if !facades.Schema().HasTable("server_network_speed") {
		return facades.Schema().Create("server_network_speed", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)                        // 关联的服务器ID
			table.Decimal("upload_speed")                   // 上传速度（KB/s）
			table.Decimal("download_speed")                 // 下载速度（KB/s）
			table.Timestamp("timestamp").UseCurrent()              // 采集时间
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000008CreateServerNetworkSpeedTable) Down() error {
	return facades.Schema().DropIfExists("server_network_speed")
}

