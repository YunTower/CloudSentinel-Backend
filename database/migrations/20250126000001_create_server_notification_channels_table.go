package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250126000001CreateServerNotificationChannelsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20250126000001CreateServerNotificationChannelsTable) Signature() string {
	return "20250126000001_create_server_notification_channels_table"
}

// Up Run the migrations.
func (r *M20250126000001CreateServerNotificationChannelsTable) Up() error {
	// 创建 server_notification_channels 表
	if !facades.Schema().HasTable("server_notification_channels") {
		err := facades.Schema().Create("server_notification_channels", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 36) // 服务器ID
			table.String("notification_type", 20) // email, webhook
			table.Boolean("enabled").Default(false) // 是否启用
			table.Timestamps()
		})
		if err != nil {
			return err
		}

		// 创建索引和唯一约束
		facades.Schema().Table("server_notification_channels", func(table schema.Blueprint) {
			table.Index("server_id")
			table.Index("notification_type")
			table.Unique("server_id", "notification_type")
			table.Foreign("server_id").References("id").On("servers") // 外键关联
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250126000001CreateServerNotificationChannelsTable) Down() error {
	// 回滚时删除表
	if facades.Schema().HasTable("server_notification_channels") {
		return facades.Schema().DropIfExists("server_notification_channels")
	}
	return nil
}

