package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260723000003AddPageIDsToIncidents struct{}

func (r *M20260723000003AddPageIDsToIncidents) Signature() string {
	return "20260723000003_add_page_ids_to_incidents"
}

func (r *M20260723000003AddPageIDsToIncidents) Up() error {
	return facades.Schema().Table("incidents", func(table schema.Blueprint) {
		// JSON 数组字符串，如 ["home"]；空/null 表示全站可见
		table.Text("page_ids").Nullable()
	})
}

func (r *M20260723000003AddPageIDsToIncidents) Down() error {
	return facades.Schema().Table("incidents", func(table schema.Blueprint) {
		table.DropColumn("page_ids")
	})
}
