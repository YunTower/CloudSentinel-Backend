package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000010CreateServerDiskIoTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000010CreateServerDiskIoTable) Signature() string {
	return "20250129000010_create_server_disk_io_table"
}

// Up Run the migrations.
func (r *M20250129000010CreateServerDiskIoTable) Up() error {
	if !facades.Schema().HasTable("server_disk_io") {
		return facades.Schema().Create("server_disk_io", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id")
			table.Decimal("read_speed")
			table.Decimal("write_speed")
			table.Timestamp("timestamp").UseCurrent()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000010CreateServerDiskIoTable) Down() error {
	return facades.Schema().DropIfExists("server_disk_io")
}
