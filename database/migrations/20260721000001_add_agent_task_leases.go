package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

// M20260721000001AddAgentTaskLeases adds an ownership-bound lease for HTTP
// task polling. A lease prevents concurrent delivery and permits safe retries.
type M20260721000001AddAgentTaskLeases struct{}

func (r *M20260721000001AddAgentTaskLeases) Signature() string {
	return "20260721000001_add_agent_task_leases"
}

func (r *M20260721000001AddAgentTaskLeases) Up() error {
	return facades.Schema().Table("agent_tasks", func(table schema.Blueprint) {
		table.String("lease_token", 36).Nullable()
		table.Timestamp("lease_expires_at").Nullable()
		table.Index("lease_token")
		table.Index("lease_expires_at")
	})
}

func (r *M20260721000001AddAgentTaskLeases) Down() error {
	return facades.Schema().Table("agent_tasks", func(table schema.Blueprint) {
		table.DropIndex("lease_token")
		table.DropIndex("lease_expires_at")
		table.DropColumn("lease_token", "lease_expires_at")
	})
}
