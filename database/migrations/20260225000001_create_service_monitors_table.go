package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

type M20260225000001CreateServiceMonitorsTable struct{}

func (r *M20260225000001CreateServiceMonitorsTable) Signature() string {
	return "20260225000001_create_service_monitors_table"
}

func (r *M20260225000001CreateServiceMonitorsTable) Up() error {
	return facades.Schema().Create("service_monitors", func(table schema.Blueprint) {
		table.ID()
		table.String("name")
		table.String("type")
		table.String("target")
		table.Integer("port").Default(0)
		table.Integer("interval").Default(60)
		table.Integer("timeout").Default(10)
		table.Boolean("enabled").Default(true)
		table.String("status").Default("unknown")
		table.Integer("response_time").Default(0)
		table.Timestamp("last_check_at").Nullable()
		table.Text("server_ids").Nullable()
		table.Integer("expect_status").Default(0)
		table.String("expect_body").Nullable()
		table.Timestamps()
	})
}

func (r *M20260225000001CreateServiceMonitorsTable) Down() error {
	return facades.Schema().DropIfExists("service_monitors")
}
