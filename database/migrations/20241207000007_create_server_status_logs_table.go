package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000007CreateServerStatusLogsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000007CreateServerStatusLogsTable) Signature() string {
	return "20241207000007_create_server_status_logs_table"
}

// Up Run the migrations.
func (r *M20241207000007CreateServerStatusLogsTable) Up() error {
	if !facades.Schema().HasTable("server_status_logs") {
		return facades.Schema().Create("server_status_logs", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)            // 关联的服务器ID
			table.String("old_status", 20).Nullable() // 变更前的状态
			table.String("new_status", 20)            // 变更后的状态
			table.Text("reason").Nullable()           // 状态变更原因
			table.Timestamp("timestamp").UseCurrent() // 状态变更时间

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000007CreateServerStatusLogsTable) Down() error {
	return facades.Schema().DropIfExists("server_status_logs")
}
