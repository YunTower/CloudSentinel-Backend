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
		&migrations.M20250129000001CreateSystemSettingsTable{},
		&migrations.M20250129000002CreateServerGroupsTable{},
		&migrations.M20250129000003CreateServersTable{},
		&migrations.M20250129000004CreateAlertNotificationsTable{},
		&migrations.M20250129000005CreateAlertRulesTable{},
		&migrations.M20250129000006CreateServerAlertRulesTable{},
		&migrations.M20250129000007CreateServerNotificationChannelsTable{},
		&migrations.M20250129000008CreateAlertsTable{},
		&migrations.M20250129000009CreateServerCpusTable{},
		&migrations.M20250129000010CreateServerDiskIoTable{},
		&migrations.M20250129000011CreateServerDisksTable{},
		&migrations.M20250129000012CreateServerMemoryHistoryTable{},
		&migrations.M20250129000013CreateServerMetricsTable{},
		&migrations.M20250129000014CreateServerNetworkConnectionsTable{},
		&migrations.M20250129000015CreateServerNetworkSpeedTable{},
		&migrations.M20250129000016CreateServerStatusLogsTable{},
		&migrations.M20250129000017CreateServerSwapTable{},
		&migrations.M20250129000018CreateServerTrafficUsageTable{},
		&migrations.M20250129000019CreateServiceMonitorAlertsTable{},
		&migrations.M20250129000020CreateServiceMonitorRuleServersTable{},
		&migrations.M20250121000001AddAgentConfigAndDisplayFlagsToServers{},
		&migrations.M20260205000001AddMonitoredServicesToServersTable{},
		&migrations.AddLogRetentionSetting{},
		&migrations.CreateAgentLogsTable{},
		&migrations.M20260206000001AddServiceStatusToServersTable{},
		&migrations.M20260206000002AddGPUInfoToServersTable{},
	}
}

func (kernel Kernel) Seeders() []seeder.Seeder {
	return []seeder.Seeder{
		&seeders.DatabaseSeeder{},
	}
}
