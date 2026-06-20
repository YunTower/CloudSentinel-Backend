package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260619000001AddHttpOptionsToServiceMonitorsTable struct{}

func (r *M20260619000001AddHttpOptionsToServiceMonitorsTable) Signature() string {
	return "20260619000001_add_http_options_to_service_monitors_table"
}

func (r *M20260619000001AddHttpOptionsToServiceMonitorsTable) Up() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.String("http_method", 10).Default("GET")
		table.Text("http_headers").Nullable()
		table.Text("http_body").Nullable()
	})
}

func (r *M20260619000001AddHttpOptionsToServiceMonitorsTable) Down() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.DropColumn("http_method", "http_headers", "http_body")
	})
}
