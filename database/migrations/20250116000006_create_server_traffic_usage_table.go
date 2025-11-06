package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250116000006CreateServerTrafficUsageTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250116000006CreateServerTrafficUsageTable) Signature() string {
	return "20250116000006_create_server_traffic_usage_table"
}

// Up Run the migrations.
func (r *M20250116000006CreateServerTrafficUsageTable) Up() error {
	if !facades.Schema().HasTable("server_traffic_usage") {
		return facades.Schema().Create("server_traffic_usage", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255)                        // 关联的服务器ID
			table.Integer("year")                                 // 年份
			table.Integer("month")                                // 月份
			table.BigInteger("upload_bytes").Default(0)            // 上传字节数
			table.BigInteger("download_bytes").Default(0)         // 下载字节数
			table.Timestamps()                                     // created_at, updated_at
			table.Unique("server_id", "year", "month")            // 唯一索引：server_id + year + month
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250116000006CreateServerTrafficUsageTable) Down() error {
	return facades.Schema().DropIfExists("server_traffic_usage")
}

