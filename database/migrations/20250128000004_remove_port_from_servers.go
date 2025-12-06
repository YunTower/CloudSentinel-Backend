package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250128000004RemovePortFromServers struct{}

// Signature The unique signature for the migration.
func (m *M20250128000004RemovePortFromServers) Signature() string {
	return "20250128000004_remove_port_from_servers"
}

// Up Run the migrations.
func (m *M20250128000004RemovePortFromServers) Up() error {
	// 从 servers 表移除 port 字段
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.DropColumn("port")
	})
}

// Down Reverse the migrations.
func (m *M20250128000004RemovePortFromServers) Down() error {
	// 重新添加 port 字段（回滚时）
	return facades.Schema().Table("servers", func(table schema.Blueprint) {
		table.Integer("port").Default(22).Nullable()
	})
}
