package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000014AddAgentKeyToServersTable struct{}

// Signature The unique signature for the migration.
func (m *M20250116000014AddAgentKeyToServersTable) Signature() string {
	return "20250116000014_add_agent_key_to_servers_table"
}

// Up Run the migrations.
func (m *M20250116000014AddAgentKeyToServersTable) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.String("agent_key", 255).Nullable()
	})
}

// Down Reverse the migrations.
func (m *M20250116000014AddAgentKeyToServersTable) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("agent_key")
	})
}

