package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000018CreateServerTrafficUsageTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000018CreateServerTrafficUsageTable) Signature() string {
	return "20250129000018_create_server_traffic_usage_table"
}

// Up Run the migrations.
func (r *M20250129000018CreateServerTrafficUsageTable) Up() error {
	if !facades.Schema().HasTable("server_traffic_usage") {
		err := facades.Schema().Create("server_traffic_usage", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id")
			table.Integer("year")
			table.Integer("month")
			table.Integer("upload_bytes").Default(0)
			table.Integer("download_bytes").Default(0)
			table.Timestamps()

			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
		if err != nil {
			return err
		}

		// 创建唯一索引
		facades.Schema().Table("server_traffic_usage", func(table schema.Blueprint) {
			table.Unique("server_traffic_usage_server_id_year_month_unique", "server_id", "year", "month")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000018CreateServerTrafficUsageTable) Down() error {
	return facades.Schema().DropIfExists("server_traffic_usage")
}
