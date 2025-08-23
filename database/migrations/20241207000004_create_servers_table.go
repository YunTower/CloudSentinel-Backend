package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000004CreateServersTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000004CreateServersTable) Signature() string {
	return "20241207000004_create_servers_table"
}

// Up Run the migrations.
func (r *M20241207000004CreateServersTable) Up() error {
	if !facades.Schema().HasTable("servers") {
		return facades.Schema().Create("servers", func(table schema.Blueprint) {
			table.String("id", 255)
			table.Primary("id")
			table.String("name", 100)
			table.String("ip", 45)
			table.Integer("port").Default(22)
			table.String("status", 20).Default("offline")
			table.String("location", 100).Nullable()
			table.String("os", 100).Nullable()
			table.String("architecture", 50).Nullable()
			table.String("kernel", 100).Nullable()
			table.String("hostname", 100).Nullable()
			table.Integer("total_disks").Default(1)
			table.Integer("cores").Default(1)
			table.Timestamps()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000004CreateServersTable) Down() error {
	return facades.Schema().DropIfExists("servers")
}
