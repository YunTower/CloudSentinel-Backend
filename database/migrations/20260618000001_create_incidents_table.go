package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260618000001CreateIncidentsTable struct{}

func (r *M20260618000001CreateIncidentsTable) Signature() string {
	return "20260618000001_create_incidents_table"
}

func (r *M20260618000001CreateIncidentsTable) Up() error {
	if !facades.Schema().HasTable("incidents") {
		err := facades.Schema().Create("incidents", func(table schema.Blueprint) {
			table.ID()
			table.String("source_type", 50)
			table.String("source_id", 100)
			table.String("title")
			table.String("status", 30).Default("active")
			table.String("impact", 30).Default("outage")
			table.Timestamp("started_at")
			table.Timestamp("resolved_at").Nullable()
			table.Timestamp("last_event_at")
			table.Timestamps()
		})
		if err != nil {
			return err
		}

		facades.Schema().Table("incidents", func(table schema.Blueprint) {
			table.Index("source_type")
			table.Index("source_id")
			table.Index("status")
			table.Index("started_at")
			table.Index("last_event_at")
		})
	}
	return nil
}

func (r *M20260618000001CreateIncidentsTable) Down() error {
	return facades.Schema().DropIfExists("incidents")
}
