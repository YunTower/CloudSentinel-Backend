package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000005CreateServerNetworkConnectionsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000005CreateServerNetworkConnectionsTable) Signature() string {
	return "20250116000005_create_server_network_connections_table"
}

// Up Run the migrations.
func (r *M20250116000005CreateServerNetworkConnectionsTable) Up() error {
	if !facades.Schema().HasTable("server_network_connections") {
		return facades.Schema().Create("server_network_connections", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)                        // 关联的服务器ID
			table.Integer("tcp_connections").Default(0)           // TCP连接数
			table.Integer("udp_connections").Default(0)           // UDP连接数
			table.Timestamp("timestamp").UseCurrent()             // 采集时间
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000005CreateServerNetworkConnectionsTable) Down() error {
	return facades.Schema().DropIfExists("server_network_connections")
}

