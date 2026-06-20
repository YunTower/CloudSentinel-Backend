package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260619000004CreateAgentTasksTable struct{}

func (r *M20260619000004CreateAgentTasksTable) Signature() string {
	return "20260619000004_create_agent_tasks_table"
}

func (r *M20260619000004CreateAgentTasksTable) Up() error {
	if facades.Schema().HasTable("agent_tasks") {
		return nil
	}
	return facades.Schema().Create("agent_tasks", func(table schema.Blueprint) {
		table.String("id")
		table.Primary("id")
		table.String("server_id")
		table.String("command", 50)
		table.String("command_id", 100).Nullable()
		table.Text("payload").Nullable()
		table.String("status", 30).Default("pending")
		table.Integer("attempts").Default(0)
		table.Text("error").Nullable()
		table.Timestamp("available_at").Nullable()
		table.Timestamp("delivered_at").Nullable()
		table.Timestamp("completed_at").Nullable()
		table.Timestamps()

		table.Foreign("server_id").References("id").On("servers")
		table.Index("server_id")
		table.Index("status")
		table.Index("available_at")
	})
}

func (r *M20260619000004CreateAgentTasksTable) Down() error {
	return facades.Schema().DropIfExists("agent_tasks")
}
