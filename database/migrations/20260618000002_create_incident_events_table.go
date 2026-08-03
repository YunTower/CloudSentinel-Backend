package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"goravel/app/facades"
)

type M20260618000002CreateIncidentEventsTable struct{}

func (r *M20260618000002CreateIncidentEventsTable) Signature() string {
	return "20260618000002_create_incident_events_table"
}

func (r *M20260618000002CreateIncidentEventsTable) Up() error {
	if !facades.Schema().HasTable("incident_events") {
		err := facades.Schema().Create("incident_events", func(table schema.Blueprint) {
			table.ID()
			table.UnsignedInteger("incident_id")
			table.String("event_type", 50)
			table.String("status", 50)
			table.Text("message")
			table.Text("metadata").Nullable()
			table.Timestamp("occurred_at")
			table.Timestamps()

			table.Foreign("incident_id").References("id").On("incidents")
		})
		if err != nil {
			return err
		}

		facades.Schema().Table("incident_events", func(table schema.Blueprint) {
			table.Index("incident_id")
			table.Index("event_type")
			table.Index("occurred_at")
		})
	}
	return nil
}

func (r *M20260618000002CreateIncidentEventsTable) Down() error {
	return facades.Schema().DropIfExists("incident_events")
}
