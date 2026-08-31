package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

// M20260830000002AddAgentKeyHashToServersTable 为 servers 增加 agent_key_hash 列，
// Agent 认证查询改走哈希索引，避免每次认证用明文密钥做 WHERE 匹配。
// 存量数据在首次认证时惰性回填。
type M20260830000002AddAgentKeyHashToServersTable struct{}

func (r *M20260830000002AddAgentKeyHashToServersTable) Signature() string {
	return "20260830000002_add_agent_key_hash_to_servers"
}

func (r *M20260830000002AddAgentKeyHashToServersTable) Up() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.String("agent_key_hash", 64).Nullable().Comment("agent_key 的 SHA-256，用于认证查询")
		table.Index("agent_key_hash")
	})
}

func (r *M20260830000002AddAgentKeyHashToServersTable) Down() error {
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("agent_key_hash")
	})
}
