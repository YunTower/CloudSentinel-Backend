package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

type M20260619000003AddProbeMetadataToServiceMonitorResultsTable struct{}

func (r *M20260619000003AddProbeMetadataToServiceMonitorResultsTable) Signature() string {
	return "20260619000003_add_probe_metadata_to_service_monitor_results_table"
}

func (r *M20260619000003AddProbeMetadataToServiceMonitorResultsTable) Up() error {
	if !facades.Schema().HasTable("service_monitor_results") {
		return nil
	}
	return facades.Schema().Table("service_monitor_results", func(table schema.Blueprint) {
		if !facades.Schema().HasColumn("service_monitor_results", "probe_name") {
			table.String("probe_name", 100).Nullable()
		}
		if !facades.Schema().HasColumn("service_monitor_results", "probe_location") {
			table.String("probe_location", 100).Nullable()
		}
	})
}

func (r *M20260619000003AddProbeMetadataToServiceMonitorResultsTable) Down() error {
	if !facades.Schema().HasTable("service_monitor_results") {
		return nil
	}
	return facades.Schema().Table("service_monitor_results", func(table schema.Blueprint) {
		if facades.Schema().HasColumn("service_monitor_results", "probe_name") {
			table.DropColumn("probe_name")
		}
		if facades.Schema().HasColumn("service_monitor_results", "probe_location") {
			table.DropColumn("probe_location")
		}
	})
}
