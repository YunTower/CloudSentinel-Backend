package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000002CreateServerGroupsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000002CreateServerGroupsTable) Signature() string {
	return "20250129000002_create_server_groups_table"
}

// Up Run the migrations.
func (r *M20250129000002CreateServerGroupsTable) Up() error {
	if !facades.Schema().HasTable("server_groups") {
		return facades.Schema().Create("server_groups", func(table schema.Blueprint) {
			table.ID()
			table.String("name")
			table.Text("description").Nullable()
			table.String("color").Nullable()
			table.Timestamps()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000002CreateServerGroupsTable) Down() error {
	return facades.Schema().DropIfExists("server_groups")
}
