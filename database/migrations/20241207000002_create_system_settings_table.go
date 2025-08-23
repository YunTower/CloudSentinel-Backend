package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000002CreateSystemSettingsTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000002CreateSystemSettingsTable) Signature() string {
	return "20241207000002_create_system_settings_table"
}

// Up Run the migrations.
func (r *M20241207000002CreateSystemSettingsTable) Up() error {
	if !facades.Schema().HasTable("system_settings") {
		return facades.Schema().Create("system_settings", func(table schema.Blueprint) {
			table.ID()
			table.String("setting_key", 100)
			table.Text("setting_value").Nullable()
			table.String("setting_type", 20).Default("string")
			table.Text("description").Nullable()
			table.Timestamps()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000002CreateSystemSettingsTable) Down() error {
	return facades.Schema().DropIfExists("system_settings")
}
