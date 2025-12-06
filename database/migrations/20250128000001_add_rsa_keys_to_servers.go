package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250128000001AddRsaKeysToServers struct{}

// Signature The unique signature for the migration.
func (m *M20250128000001AddRsaKeysToServers) Signature() string {
	return "20250128000001_add_rsa_keys_to_servers"
}

// Up Run the migrations.
func (m *M20250128000001AddRsaKeysToServers) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		// 面板私钥（加密存储）
		table.Text("panel_private_key").Nullable()
		// 面板公钥
		table.Text("panel_public_key").Nullable()
		// Agent 公钥
		table.Text("agent_public_key").Nullable()
		// Agent 公钥指纹（SHA256，64字符）
		table.String("agent_fingerprint", 64).Nullable()
	})
}

// Down Reverse the migrations.
func (m *M20250128000001AddRsaKeysToServers) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("panel_private_key")
		table.DropColumn("panel_public_key")
		table.DropColumn("agent_public_key")
		table.DropColumn("agent_fingerprint")
	})
}
