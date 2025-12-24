package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000017CreateServerSwapTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000017CreateServerSwapTable) Signature() string {
	return "20250129000017_create_server_swap_table"
}

// Up Run the migrations.
func (r *M20250129000017CreateServerSwapTable) Up() error {
	if !facades.Schema().HasTable("server_swap") {
		err := facades.Schema().Create("server_swap", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)
			table.BigInteger("swap_total")
			table.BigInteger("swap_used")
			table.BigInteger("swap_free")
			table.Timestamp("timestamp").UseCurrent()
			table.Timestamps()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
		if err != nil {
			return err
		}

		// 创建索引
		facades.Schema().Table("server_swap", func(table schema.Blueprint) {
			table.Index("server_id")
			table.Index("timestamp")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000017CreateServerSwapTable) Down() error {
	return facades.Schema().DropIfExists("server_swap")
}
