package database

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/database/seeder"

	"goravel/database/migrations"
	"goravel/database/seeders"
)

type Kernel struct {
}

func (kernel Kernel) Migrations() []schema.Migration {
	return []schema.Migration{

		&migrations.M20241207000002CreateSystemSettingsTable{},
		&migrations.M20241207000004CreateServersTable{},
		&migrations.M20241207000005CreateServerDisksTable{},
		&migrations.M20241207000006CreateServerMetricsTable{},
		&migrations.M20241207000007CreateServerStatusLogsTable{},
		&migrations.M20241207000008CreateAlertRulesTable{},
		&migrations.M20241207000009CreateAlertsTable{},
		&migrations.M20241207000010CreateAlertNotificationsTable{},
		&migrations.M20241207000011CreateMonitorConfigTable{},
		&migrations.M20241207000013CreateIndexesTable{},
	}
}

func (kernel Kernel) Seeders() []seeder.Seeder {
	return []seeder.Seeder{
		&seeders.DatabaseSeeder{},
	}
}
