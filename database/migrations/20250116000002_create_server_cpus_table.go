package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000002CreateServerCpusTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000002CreateServerCpusTable) Signature() string {
	return "20250116000002_create_server_cpus_table"
}

// Up Run the migrations.
func (r *M20250116000002CreateServerCpusTable) Up() error {
	if !facades.Schema().HasTable("server_cpus") {
		return facades.Schema().Create("server_cpus", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)                    // 关联的服务器ID
			table.String("cpu_name", 200)                    // CPU名称
			table.Decimal("cpu_usage")                  // CPU占用率（百分比）
			table.Integer("cores")                            // 核心数
			table.Timestamp("timestamp").UseCurrent()         // 采集时间
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000002CreateServerCpusTable) Down() error {
	return facades.Schema().DropIfExists("server_cpus")
}

