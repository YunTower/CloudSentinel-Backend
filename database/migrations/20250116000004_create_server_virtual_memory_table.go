package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000004CreateServerVirtualMemoryTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000004CreateServerVirtualMemoryTable) Signature() string {
	return "20250116000004_create_server_virtual_memory_table"
}

// Up Run the migrations.
func (r *M20250116000004CreateServerVirtualMemoryTable) Up() error {
	if !facades.Schema().HasTable("server_virtual_memory") {
		return facades.Schema().Create("server_virtual_memory", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)                        // 关联的服务器ID
			table.BigInteger("virtual_memory_total")               // 虚拟内存总量（字节）
			table.BigInteger("virtual_memory_used")               // 已使用虚拟内存（字节）
			table.BigInteger("virtual_memory_free")                // 空闲虚拟内存（字节）
			table.Timestamp("timestamp").UseCurrent()              // 采集时间
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000004CreateServerVirtualMemoryTable) Down() error {
	return facades.Schema().DropIfExists("server_virtual_memory")
}

