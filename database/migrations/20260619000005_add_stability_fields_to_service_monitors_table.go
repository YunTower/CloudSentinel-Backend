package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260619000005AddStabilityFieldsToServiceMonitorsTable struct{}

func (r *M20260619000005AddStabilityFieldsToServiceMonitorsTable) Signature() string {
	return "20260619000005_add_stability_fields_to_service_monitors_table"
}

func (r *M20260619000005AddStabilityFieldsToServiceMonitorsTable) Up() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.Integer("failure_threshold").Default(1)
		table.Integer("recovery_threshold").Default(1)
		table.Integer("consecutive_failures").Default(0)
		table.Integer("consecutive_successes").Default(0)
	})
}

func (r *M20260619000005AddStabilityFieldsToServiceMonitorsTable) Down() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.DropColumn("failure_threshold", "recovery_threshold", "consecutive_failures", "consecutive_successes")
	})
}
