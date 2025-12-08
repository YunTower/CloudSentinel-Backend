package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000016CreateServerStatusLogsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000016CreateServerStatusLogsTable) Signature() string {
	return "20250129000016_create_server_status_logs_table"
}

// Up Run the migrations.
func (r *M20250129000016CreateServerStatusLogsTable) Up() error {
	if !facades.Schema().HasTable("server_status_logs") {
		return facades.Schema().Create("server_status_logs", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id").NotNull()
			table.String("old_status").Nullable()
			table.String("new_status").NotNull()
			table.Text("reason").Nullable()
			table.Timestamp("timestamp").UseCurrent().NotNull()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000016CreateServerStatusLogsTable) Down() error {
	return facades.Schema().DropIfExists("server_status_logs")
}
