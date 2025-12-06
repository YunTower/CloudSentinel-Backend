package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000017RenameVirtualMemoryToSwap struct{}

// Signature The unique signature for the migration.
func (r *M20250116000017RenameVirtualMemoryToSwap) Signature() string {
	return "20250116000017_rename_virtual_memory_to_swap"
}

// Up Run the migrations.
func (r *M20250116000017RenameVirtualMemoryToSwap) Up() error {
	// 如果表存在，重命名表（SQLite 不支持直接重命名字段，字段重命名由后续迁移处理）
	if facades.Schema().HasTable("server_virtual_memory") {
		// 重命名表
		_, _ = facades.Orm().Query().Exec("ALTER TABLE server_virtual_memory RENAME TO server_swap")

		// 注意：SQLite 不支持 CHANGE COLUMN，字段重命名由 20250128000005_fix_server_swap_columns 迁移处理
		// 这里只重命名表，字段保持原样，后续迁移会处理字段重命名
	} else if !facades.Schema().HasTable("server_swap") {
		// 如果表不存在，创建新表
		return facades.Schema().Create("server_swap", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)            // 关联的服务器ID
			table.BigInteger("swap_total")            // Swap总量（字节）
			table.BigInteger("swap_used")             // 已使用Swap（字节）
			table.BigInteger("swap_free")             // 空闲Swap（字节）
			table.Timestamp("timestamp").UseCurrent() // 采集时间
			table.Timestamps()
			table.Foreign("server_id").References("id").On("servers") // 外键关联
			table.Index("idx_server_swap_server_id", "server_id")
			table.Index("idx_server_swap_timestamp", "timestamp")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000017RenameVirtualMemoryToSwap) Down() error {
	// 如果表存在，重命名回原来的名称
	if facades.Schema().HasTable("server_swap") {
		// 重命名字段
		facades.Orm().Query().Exec("ALTER TABLE server_swap CHANGE COLUMN swap_total virtual_memory_total BIGINT")
		facades.Orm().Query().Exec("ALTER TABLE server_swap CHANGE COLUMN swap_used virtual_memory_used BIGINT")
		facades.Orm().Query().Exec("ALTER TABLE server_swap CHANGE COLUMN swap_free virtual_memory_free BIGINT")

		// 重命名索引（先删除新索引，再创建旧索引）
		facades.Orm().Query().Exec("DROP INDEX IF EXISTS idx_server_swap_server_id ON server_swap")
		facades.Orm().Query().Exec("DROP INDEX IF EXISTS idx_server_swap_timestamp ON server_swap")
		facades.Orm().Query().Exec("CREATE INDEX idx_server_virtual_memory_server_id ON server_swap(server_id)")
		facades.Orm().Query().Exec("CREATE INDEX idx_server_virtual_memory_timestamp ON server_swap(timestamp)")

		// 重命名表
		facades.Orm().Query().Exec("ALTER TABLE server_swap RENAME TO server_virtual_memory")
	}

	return nil
}
