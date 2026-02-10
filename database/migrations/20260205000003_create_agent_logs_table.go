package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type CreateAgentLogsTable struct {
}

func (r *CreateAgentLogsTable) Signature() string {
	return "20260205000003_create_agent_logs_table"
}

func (r *CreateAgentLogsTable) Up() error {
	return facades.Schema().Create("agent_logs", func(table schema.Blueprint) {
		table.ID()
		table.String("server_id")
		table.String("level").Comment("日志级别: info, warn, error")
		table.Text("message").Comment("日志内容")
		table.Text("context").Nullable().Comment("上下文信息(JSON)")
		table.Timestamp("created_at").UseCurrent()

		table.Index("server_id")
		table.Index("created_at")
	})
}

func (r *CreateAgentLogsTable) Down() error {
	return facades.Schema().DropIfExists("agent_logs")
}
