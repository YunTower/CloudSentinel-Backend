package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250128000002FixMissingTimestamps struct{}

// Signature The unique signature for the migration.
func (m *M20250128000002FixMissingTimestamps) Signature() string {
	return "20250128000002_fix_missing_timestamps"
}

// Up Run the migrations.
func (m *M20250128000002FixMissingTimestamps) Up() error {
	// 修复 server_memory_history 表
	if facades.Schema().HasTable("server_memory_history") {
		if !facades.Schema().HasColumn("server_memory_history", "created_at") {
			err := facades.Schema().Table("server_memory_history", func(table schema.Blueprint) {
				table.Timestamp("created_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
		if !facades.Schema().HasColumn("server_memory_history", "updated_at") {
			err := facades.Schema().Table("server_memory_history", func(table schema.Blueprint) {
				table.Timestamp("updated_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
	}

	// 修复 server_metrics 表
	if facades.Schema().HasTable("server_metrics") {
		if !facades.Schema().HasColumn("server_metrics", "created_at") {
			err := facades.Schema().Table("server_metrics", func(table schema.Blueprint) {
				table.Timestamp("created_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
		if !facades.Schema().HasColumn("server_metrics", "updated_at") {
			err := facades.Schema().Table("server_metrics", func(table schema.Blueprint) {
				table.Timestamp("updated_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
	}

	// 修复 server_network_speed 表
	if facades.Schema().HasTable("server_network_speed") {
		if !facades.Schema().HasColumn("server_network_speed", "created_at") {
			err := facades.Schema().Table("server_network_speed", func(table schema.Blueprint) {
				table.Timestamp("created_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
		if !facades.Schema().HasColumn("server_network_speed", "updated_at") {
			err := facades.Schema().Table("server_network_speed", func(table schema.Blueprint) {
				table.Timestamp("updated_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
	}

	// 修复 server_swap 表 - 检查并添加缺失的字段
	if facades.Schema().HasTable("server_swap") {
		// 检查是否有 swap_total 字段（可能表结构不正确）
		if !facades.Schema().HasColumn("server_swap", "swap_total") {
			// 如果表存在但没有 swap_total，可能是旧表结构，需要添加字段
			err := facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.BigInteger("swap_total").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
		if !facades.Schema().HasColumn("server_swap", "swap_used") {
			err := facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.BigInteger("swap_used").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
		if !facades.Schema().HasColumn("server_swap", "swap_free") {
			err := facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.BigInteger("swap_free").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
		if !facades.Schema().HasColumn("server_swap", "created_at") {
			err := facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.Timestamp("created_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
		if !facades.Schema().HasColumn("server_swap", "updated_at") {
			err := facades.Schema().Table("server_swap", func(table schema.Blueprint) {
				table.Timestamp("updated_at").Nullable()
			})
			if err != nil {
				// 忽略错误，可能列已存在
			}
		}
	}

	return nil
}

// Down Reverse the migrations.
func (m *M20250128000002FixMissingTimestamps) Down() error {
	// 回滚时删除添加的字段
	if facades.Schema().HasTable("server_memory_history") {
		facades.Schema().Table("server_memory_history", func(table schema.Blueprint) {
			table.DropColumn("created_at")
			table.DropColumn("updated_at")
		})
	}

	if facades.Schema().HasTable("server_metrics") {
		facades.Schema().Table("server_metrics", func(table schema.Blueprint) {
			table.DropColumn("created_at")
			table.DropColumn("updated_at")
		})
	}

	if facades.Schema().HasTable("server_network_speed") {
		facades.Schema().Table("server_network_speed", func(table schema.Blueprint) {
			table.DropColumn("created_at")
			table.DropColumn("updated_at")
		})
	}

	// 注意：server_swap 的字段不回滚，因为可能是修复表结构

	return nil
}
