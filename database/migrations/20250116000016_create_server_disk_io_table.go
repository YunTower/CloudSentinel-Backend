package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000016CreateServerDiskIoTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000016CreateServerDiskIoTable) Signature() string {
	return "20250116000016_create_server_disk_io_table"
}

// Up Run the migrations.
func (r *M20250116000016CreateServerDiskIoTable) Up() error {
	if !facades.Schema().HasTable("server_disk_io") {
		return facades.Schema().Create("server_disk_io", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)                        // 关联的服务器ID
			table.Decimal("read_speed")                           // 读取速度（KB/s）
			table.Decimal("write_speed")                          // 写入速度（KB/s）
			table.Timestamp("timestamp").UseCurrent()            // 采集时间
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000016CreateServerDiskIoTable) Down() error {
	return facades.Schema().DropIfExists("server_disk_io")
}

