package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260205000001AddMonitoredServicesToServersTable struct{}

// Signature The unique signature for the migration.
func (r *M20260205000001AddMonitoredServicesToServersTable) Signature() string {
	return "20260205000001_add_monitored_services_to_servers_table"
}

// Up Run the migrations.
func (r *M20260205000001AddMonitoredServicesToServersTable) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.Text("monitored_services").Nullable().Comment("监控的服务列表(JSON)")
	})
}

// Down Reverse the migrations.
func (r *M20260205000001AddMonitoredServicesToServersTable) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("monitored_services")
	})
}
