package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250128000005FixServerSwapColumns struct{}

// Signature The unique signature for the migration.
func (m *M20250128000005FixServerSwapColumns) Signature() string {
	return "20250128000005_fix_server_swap_columns"
}

// Up Run the migrations.
func (m *M20250128000005FixServerSwapColumns) Up() error {
	// 检查表是否存在
	if !facades.Schema().HasTable("server_swap") {
		// 如果表不存在，创建新表
		return facades.Schema().Create("server_swap", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)
			table.BigInteger("swap_total")
			table.BigInteger("swap_used")
			table.BigInteger("swap_free")
			table.Timestamp("timestamp").UseCurrent()
			table.Timestamps()
			table.Foreign("server_id").References("id").On("servers")
			table.Index("idx_server_swap_server_id", "server_id")
			table.Index("idx_server_swap_timestamp", "timestamp")
		})
	}

	// 检查是否有旧的 virtual_memory_* 字段和新字段
	hasOldColumns := facades.Schema().HasColumn("server_swap", "virtual_memory_total") ||
		facades.Schema().HasColumn("server_swap", "virtual_memory_used") ||
		facades.Schema().HasColumn("server_swap", "virtual_memory_free")
	hasNewColumns := facades.Schema().HasColumn("server_swap", "swap_total") ||
		facades.Schema().HasColumn("server_swap", "swap_used") ||
		facades.Schema().HasColumn("server_swap", "swap_free")

	// 如果有旧字段，需要重建表（SQLite 不支持直接重命名字段）
	if hasOldColumns {
		// 1. 创建临时表（只包含新字段）
		_, err := facades.Orm().Query().Exec(`
			CREATE TABLE server_swap_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				server_id VARCHAR(255) NOT NULL,
				swap_total BIGINT NOT NULL,
				swap_used BIGINT NOT NULL,
				swap_free BIGINT NOT NULL,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				created_at DATETIME,
				updated_at DATETIME,
				FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
			)
		`)
		if err != nil {
			return err
		}

		// 2. 复制数据：优先使用新字段，如果新字段不存在则使用旧字段迁移
		var sqlQuery string
		if hasNewColumns {
			// 如果新字段已存在，优先使用新字段的值
			sqlQuery = `
				INSERT INTO server_swap_new (id, server_id, swap_total, swap_used, swap_free, timestamp, created_at, updated_at)
				SELECT 
					id,
					server_id,
					COALESCE(swap_total, 0) as swap_total,
					COALESCE(swap_used, 0) as swap_used,
					COALESCE(swap_free, 0) as swap_free,
					COALESCE(timestamp, datetime('now')) as timestamp,
					COALESCE(created_at, datetime('now')) as created_at,
					COALESCE(updated_at, datetime('now')) as updated_at
				FROM server_swap
			`
		} else {
			// 如果新字段不存在，从旧字段迁移数据
			sqlQuery = `
				INSERT INTO server_swap_new (id, server_id, swap_total, swap_used, swap_free, timestamp, created_at, updated_at)
				SELECT 
					id,
					server_id,
					COALESCE(virtual_memory_total, 0) as swap_total,
					COALESCE(virtual_memory_used, 0) as swap_used,
					COALESCE(virtual_memory_free, 0) as swap_free,
					COALESCE(timestamp, datetime('now')) as timestamp,
					COALESCE(created_at, datetime('now')) as created_at,
					COALESCE(updated_at, datetime('now')) as updated_at
				FROM server_swap
			`
		}
		_, err = facades.Orm().Query().Exec(sqlQuery)
		if err != nil {
			// 如果迁移失败，删除临时表
			facades.Orm().Query().Exec("DROP TABLE IF EXISTS server_swap_new")
			return err
		}

		// 3. 删除旧表
		_, err = facades.Orm().Query().Exec("DROP TABLE server_swap")
		if err != nil {
			return err
		}

		// 4. 重命名新表
		_, err = facades.Orm().Query().Exec("ALTER TABLE server_swap_new RENAME TO server_swap")
		if err != nil {
			return err
		}

		// 5. 创建索引
		_, _ = facades.Orm().Query().Exec("CREATE INDEX IF NOT EXISTS idx_server_swap_server_id ON server_swap(server_id)")
		_, _ = facades.Orm().Query().Exec("CREATE INDEX IF NOT EXISTS idx_server_swap_timestamp ON server_swap(timestamp)")
	} else {
		// 如果没有旧字段，只需要确保新字段存在
		if !facades.Schema().HasColumn("server_swap", "swap_total") {
			_ = facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.BigInteger("swap_total").Nullable()
			})
		}
		if !facades.Schema().HasColumn("server_swap", "swap_used") {
			_ = facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.BigInteger("swap_used").Nullable()
			})
		}
		if !facades.Schema().HasColumn("server_swap", "swap_free") {
			_ = facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.BigInteger("swap_free").Nullable()
			})
		}
	}

	return nil
}

// Down Reverse the migrations.
func (m *M20250128000005FixServerSwapColumns) Down() error {
	// 回滚操作：如果需要，可以恢复旧字段名
	// 但通常不需要回滚这个修复
	return nil
}
