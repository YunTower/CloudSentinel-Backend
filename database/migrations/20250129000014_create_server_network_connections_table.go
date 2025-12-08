package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000014CreateServerNetworkConnectionsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000014CreateServerNetworkConnectionsTable) Signature() string {
	return "20250129000014_create_server_network_connections_table"
}

// Up Run the migrations.
func (r *M20250129000014CreateServerNetworkConnectionsTable) Up() error {
	if !facades.Schema().HasTable("server_network_connections") {
		return facades.Schema().Create("server_network_connections", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id").NotNull()
			table.Integer("tcp_connections").Default(0).NotNull()
			table.Integer("udp_connections").Default(0).NotNull()
			table.Timestamp("timestamp").UseCurrent().NotNull()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000014CreateServerNetworkConnectionsTable) Down() error {
	return facades.Schema().DropIfExists("server_network_connections")
}
