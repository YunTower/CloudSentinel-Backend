package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260619000002AddGroupNameToServiceMonitorsTable struct{}

func (r *M20260619000002AddGroupNameToServiceMonitorsTable) Signature() string {
	return "20260619000002_add_group_name_to_service_monitors_table"
}

func (r *M20260619000002AddGroupNameToServiceMonitorsTable) Up() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.String("group_name", 100).Nullable()
	})
}

func (r *M20260619000002AddGroupNameToServiceMonitorsTable) Down() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.DropColumn("group_name")
	})
}
