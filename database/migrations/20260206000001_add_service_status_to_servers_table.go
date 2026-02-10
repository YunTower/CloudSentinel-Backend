package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260206000001AddServiceStatusToServersTable struct{}

// Signature The unique signature for the migration.
func (r *M20260206000001AddServiceStatusToServersTable) Signature() string {
	return "20260206000001_add_service_status_to_servers_table"
}

// Up Run the migrations.
func (r *M20260206000001AddServiceStatusToServersTable) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.Text("service_status").Nullable().Comment("服务监控状态(JSON)")
	})
}

// Down Reverse the migrations.
func (r *M20260206000001AddServiceStatusToServersTable) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("service_status")
	})
}
