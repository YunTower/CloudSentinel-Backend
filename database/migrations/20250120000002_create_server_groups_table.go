package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250120000002CreateServerGroupsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250120000002CreateServerGroupsTable) Signature() string {
	return "20250120000002_create_server_groups_table"
}

// Up Run the migrations.
func (r *M20250120000002CreateServerGroupsTable) Up() error {
	if !facades.Schema().HasTable("server_groups") {
		return facades.Schema().Create("server_groups", func(table schema.Blueprint) {
			table.ID()
			table.String("name", 100)            // 分组名称
			table.Text("description").Nullable() // 分组描述
			table.String("color", 20).Nullable() // 分组颜色标识（可选）
			table.Timestamps()                   // created_at, updated_at
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250120000002CreateServerGroupsTable) Down() error {
	return facades.Schema().DropIfExists("server_groups")
}
