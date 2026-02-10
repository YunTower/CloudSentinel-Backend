package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260206000002AddGPUInfoToServersTable struct{}

// Signature The unique signature for the migration.
func (r *M20260206000002AddGPUInfoToServersTable) Signature() string {
	return "20260206000002_add_gpu_info_to_servers_table"
}

// Up Run the migrations.
func (r *M20260206000002AddGPUInfoToServersTable) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.Text("gpu_info").Nullable().Comment("GPU信息(JSON)")
	})
}

// Down Reverse the migrations.
func (r *M20260206000002AddGPUInfoToServersTable) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("gpu_info")
	})
}
