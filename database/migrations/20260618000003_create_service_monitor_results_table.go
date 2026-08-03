package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

type M20260618000003CreateServiceMonitorResultsTable struct{}

func (r *M20260618000003CreateServiceMonitorResultsTable) Signature() string {
	return "20260618000003_create_service_monitor_results_table"
}

func (r *M20260618000003CreateServiceMonitorResultsTable) Up() error {
	if !facades.Schema().HasTable("service_monitor_results") {
		err := facades.Schema().Create("service_monitor_results", func(table schema.Blueprint) {
			table.ID()
			table.UnsignedInteger("monitor_id")
			table.String("probe_type", 30)
			table.String("probe_id", 100).Nullable()
			table.String("status", 30)
			table.Integer("response_time").Default(0)
			table.Text("error").Nullable()
			table.Timestamp("checked_at")
			table.Timestamps()

			table.Foreign("monitor_id").References("id").On("service_monitors")
		})
		if err != nil {
			return err
		}

		facades.Schema().Table("service_monitor_results", func(table schema.Blueprint) {
			table.Index("monitor_id")
			table.Index("probe_type")
			table.Index("probe_id")
			table.Index("status")
			table.Index("checked_at")
		})
	}
	return nil
}

func (r *M20260618000003CreateServiceMonitorResultsTable) Down() error {
	return facades.Schema().DropIfExists("service_monitor_results")
}
