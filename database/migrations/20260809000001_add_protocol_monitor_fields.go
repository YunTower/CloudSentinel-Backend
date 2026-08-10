package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

type M20260809000001AddProtocolMonitorFields struct{}

func (r *M20260809000001AddProtocolMonitorFields) Signature() string {
	return "20260809000001_add_protocol_monitor_fields"
}

func (r *M20260809000001AddProtocolMonitorFields) Up() error {
	if err := facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.String("ai_api_format").Nullable()
		table.String("ai_model").Nullable()
		table.Text("ai_api_key_encrypted").Nullable()
		table.Text("last_metadata").Nullable()
		table.Timestamp("metadata_checked_at").Nullable()
	}); err != nil {
		return err
	}
	return facades.Schema().Table("service_monitor_results", func(table schema.Blueprint) {
		table.String("error_code").Nullable()
		table.Text("metadata").Nullable()
	})
}

func (r *M20260809000001AddProtocolMonitorFields) Down() error {
	if err := facades.Schema().Table("service_monitor_results", func(table schema.Blueprint) {
		table.DropColumn("error_code")
		table.DropColumn("metadata")
	}); err != nil {
		return err
	}
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.DropColumn("ai_api_format")
		table.DropColumn("ai_model")
		table.DropColumn("ai_api_key_encrypted")
		table.DropColumn("last_metadata")
		table.DropColumn("metadata_checked_at")
	})
}
