package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000003CreateServerMemoryHistoryTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000003CreateServerMemoryHistoryTable) Signature() string {
	return "20250116000003_create_server_memory_history_table"
}

// Up Run the migrations.
func (r *M20250116000003CreateServerMemoryHistoryTable) Up() error {
	if !facades.Schema().HasTable("server_memory_history") {
		return facades.Schema().Create("server_memory_history", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)                        // 关联的服务器ID
			table.BigInteger("memory_total")                      // 内存总量（字节）
			table.BigInteger("memory_used")                       // 已使用内存（字节）
			table.Decimal("memory_usage_percent")            // 内存使用率（百分比）
			table.Timestamp("timestamp").UseCurrent()             // 采集时间
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000003CreateServerMemoryHistoryTable) Down() error {
	return facades.Schema().DropIfExists("server_memory_history")
}

