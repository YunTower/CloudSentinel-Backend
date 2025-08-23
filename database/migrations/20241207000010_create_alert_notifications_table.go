package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000010CreateAlertNotificationsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000010CreateAlertNotificationsTable) Signature() string {
	return "20241207000010_create_alert_notifications_table"
}

// Up Run the migrations.
func (r *M20241207000010CreateAlertNotificationsTable) Up() error {
	if !facades.Schema().HasTable("alert_notifications") {
		return facades.Schema().Create("alert_notifications", func(table schema.Blueprint) {
			table.ID()
			table.String("notification_type", 20)   // 通知类型
			table.Boolean("enabled").Default(false) // 是否启用该通知方式
			table.Text("config_json")               // 通知配置（JSON格式存储）
			table.Timestamps()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000010CreateAlertNotificationsTable) Down() error {
	return facades.Schema().DropIfExists("alert_notifications")
}
