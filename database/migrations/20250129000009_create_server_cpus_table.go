package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000009CreateServerCpusTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000009CreateServerCpusTable) Signature() string {
	return "20250129000009_create_server_cpus_table"
}

// Up Run the migrations.
func (r *M20250129000009CreateServerCpusTable) Up() error {
	if !facades.Schema().HasTable("server_cpus") {
		return facades.Schema().Create("server_cpus", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id").NotNull()
			table.String("cpu_name").NotNull()
			table.Decimal("cpu_usage").NotNull()
			table.Integer("cores").NotNull()
			table.Timestamp("timestamp").UseCurrent().NotNull()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000009CreateServerCpusTable) Down() error {
	return facades.Schema().DropIfExists("server_cpus")
}
