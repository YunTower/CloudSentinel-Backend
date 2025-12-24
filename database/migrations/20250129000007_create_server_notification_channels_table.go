package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000007CreateServerNotificationChannelsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000007CreateServerNotificationChannelsTable) Signature() string {
	return "20250129000007_create_server_notification_channels_table"
}

// Up Run the migrations.
func (r *M20250129000007CreateServerNotificationChannelsTable) Up() error {
	if !facades.Schema().HasTable("server_notification_channels") {
		err := facades.Schema().Create("server_notification_channels", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id")
			table.String("notification_type")
			table.Boolean("enabled").Default(false)
			table.Timestamps()
		})
		if err != nil {
			return err
		}

		// 创建索引
		facades.Schema().Table("server_notification_channels", func(table schema.Blueprint) {
			table.Index("server_id")
			table.Index("notification_type")
			table.Unique("server_id", "notification_type")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000007CreateServerNotificationChannelsTable) Down() error {
	return facades.Schema().DropIfExists("server_notification_channels")
}
