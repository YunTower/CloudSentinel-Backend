package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

// M20260830000001CreateAlertStatesTable 持久化告警状态与最近通知时间，
// 替代此前的进程内缓存（重启即丢，导致重复告警/丢失恢复通知）。
type M20260830000001CreateAlertStatesTable struct{}

func (r *M20260830000001CreateAlertStatesTable) Signature() string {
	return "20260830000001_create_alert_states_table"
}

func (r *M20260830000001CreateAlertStatesTable) Up() error {
	if facades.Schema().HasTable("alert_states") {
		return nil
	}

	return facades.Schema().Create("alert_states", func(table schema.Blueprint) {
		table.ID()
		table.String("server_id", 64).Default("")
		table.String("metric", 64).Default("")
		table.String("state", 20).Default("normal")
		table.Timestamp("last_notified_at").Nullable()
		table.Timestamp("created_at").Nullable()
		table.Timestamp("updated_at").Nullable()
		table.Unique("server_id", "metric")
	})
}

func (r *M20260830000001CreateAlertStatesTable) Down() error {
	return facades.Schema().DropIfExists("alert_states")
}
