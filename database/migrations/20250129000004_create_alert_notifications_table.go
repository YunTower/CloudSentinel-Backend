package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20250129000004CreateAlertNotificationsTable struct{}

// Signature The unique signature for the migration.
func (r *M20250129000004CreateAlertNotificationsTable) Signature() string {
	return "20250129000004_create_alert_notifications_table"
}

// Up Run the migrations.
func (r *M20250129000004CreateAlertNotificationsTable) Up() error {
	if !facades.Schema().HasTable("alert_notifications") {
		return facades.Schema().Create("alert_notifications", func(table schema.Blueprint) {
			table.ID()
			table.String("notification_type")
			table.Boolean("enabled").Default(false)
			table.Text("config_json")
			table.Timestamps()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20250129000004CreateAlertNotificationsTable) Down() error {
	return facades.Schema().DropIfExists("alert_notifications")
}
