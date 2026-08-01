package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260730000001AddCpuNameToServersTable struct{}

// Signature The unique signature for the migration.
func (r *M20260730000001AddCpuNameToServersTable) Signature() string {
	return "20260730000001_add_cpu_name_to_servers_table"
}

// Up Run the migrations.
func (r *M20260730000001AddCpuNameToServersTable) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.String("cpu_name", 255).Nullable().Comment("CPU 型号")
	})
}

// Down Reverse the migrations.
func (r *M20260730000001AddCpuNameToServersTable) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("cpu_name")
	})
}
