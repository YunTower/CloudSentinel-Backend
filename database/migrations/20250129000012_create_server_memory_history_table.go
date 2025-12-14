package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000012CreateServerMemoryHistoryTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000012CreateServerMemoryHistoryTable) Signature() string {
	return "20250129000012_create_server_memory_history_table"
}

// Up Run the migrations.
func (r *M20250129000012CreateServerMemoryHistoryTable) Up() error {
	if !facades.Schema().HasTable("server_memory_history") {
		return facades.Schema().Create("server_memory_history", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id")
			table.Integer("memory_total")
			table.Integer("memory_used")
			table.Decimal("memory_usage_percent")
			table.Timestamp("timestamp").UseCurrent()
			table.Timestamps()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000012CreateServerMemoryHistoryTable) Down() error {
	return facades.Schema().DropIfExists("server_memory_history")
}
