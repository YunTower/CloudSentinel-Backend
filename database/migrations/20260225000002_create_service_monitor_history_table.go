package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260225000002CreateServiceMonitorHistoryTable struct{}

func (r *M20260225000002CreateServiceMonitorHistoryTable) Signature() string {
	return "20260225000002_create_service_monitor_history_table"
}

func (r *M20260225000002CreateServiceMonitorHistoryTable) Up() error {
	return facades.Schema().Create("service_monitor_history", func(table schema.Blueprint) {
		table.ID()
		table.UnsignedInteger("monitor_id")
		table.String("status") // up, slow, down
		table.Integer("response_time").Default(0)
		table.Timestamp("checked_at")
		table.Index("monitor_id")
		table.Index("checked_at")
	})
}

func (r *M20260225000002CreateServiceMonitorHistoryTable) Down() error {
	return facades.Schema().DropIfExists("service_monitor_history")
}
