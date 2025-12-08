package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000011CreateServerDisksTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000011CreateServerDisksTable) Signature() string {
	return "20250129000011_create_server_disks_table"
}

// Up Run the migrations.
func (r *M20250129000011CreateServerDisksTable) Up() error {
	if !facades.Schema().HasTable("server_disks") {
		return facades.Schema().Create("server_disks", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id").NotNull()
			table.String("disk_name").NotNull()
			table.String("mount_point").Nullable()
			table.String("filesystem").Nullable()
			table.Integer("total_size").NotNull()
			table.Integer("used_size").Default(0).NotNull()
			table.Integer("free_size").Default(0).NotNull()
			table.String("disk_type", 50).Default("unknown").NotNull()
			table.Boolean("is_boot").Default(false).NotNull()
			table.Timestamps()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000011CreateServerDisksTable) Down() error {
	return facades.Schema().DropIfExists("server_disks")
}
