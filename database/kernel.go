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
		&migrations.M20241207000009CreateAlertsTable{},
		&migrations.M20241207000010CreateAlertNotificationsTable{},
		&migrations.M20241207000011CreateMonitorConfigTable{},
		&migrations.M20250116000001ExtendServersTable{},
		&migrations.M20250116000002CreateServerCpusTable{},
		&migrations.M20250116000003CreateServerMemoryHistoryTable{},
		&migrations.M20250116000004CreateServerVirtualMemoryTable{},
		&migrations.M20250116000005CreateServerNetworkConnectionsTable{},
		&migrations.M20250116000006CreateServerTrafficUsageTable{},
		&migrations.M20250116000007CreateTrafficResetConfigTable{},
		&migrations.M20250116000008CreateServerNetworkSpeedTable{},
		&migrations.M20250116000009ReplaceAlertRulesTable{},
		&migrations.M20250116000010CreateServiceMonitorRuleServersTable{},
		&migrations.M20250116000011CreateServiceMonitorAlertsTable{},
		&migrations.M20250116000012CreateLogCleanupConfigTable{},
		&migrations.M20250116000013CreateNewIndexesTable{},
		&migrations.M20250116000014AddAgentKeyToServersTable{},
		&migrations.M20250116000016CreateServerDiskIoTable{},
	}
}

func (kernel Kernel) Seeders() []seeder.Seeder {
	return []seeder.Seeder{
		&seeders.DatabaseSeeder{},
	}
}
