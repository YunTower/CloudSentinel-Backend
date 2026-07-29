package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20260723000002AddCertExpiryToServiceMonitors struct{}

func (r *M20260723000002AddCertExpiryToServiceMonitors) Signature() string {
	return "20260723000002_add_cert_expiry_to_service_monitors"
}

func (r *M20260723000002AddCertExpiryToServiceMonitors) Up() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.Boolean("check_cert_expiry").Default(false)
		table.Timestamp("cert_expires_at").Nullable()
		table.Integer("cert_days_left").Nullable()
	})
}

func (r *M20260723000002AddCertExpiryToServiceMonitors) Down() error {
	return facades.Schema().Table("service_monitors", func(table schema.Blueprint) {
		table.DropColumn("check_cert_expiry")
		table.DropColumn("cert_expires_at")
		table.DropColumn("cert_days_left")
	})
}
