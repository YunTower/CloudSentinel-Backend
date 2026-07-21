package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

// M20260721000002CreateRateLimitBuckets stores short-lived counters in the
// existing application database. This keeps rate limits shared by all local
// backend processes without adding a Redis or proxy dependency.
type M20260721000002CreateRateLimitBuckets struct{}

func (r *M20260721000002CreateRateLimitBuckets) Signature() string {
	return "20260721000002_create_rate_limit_buckets"
}

func (r *M20260721000002CreateRateLimitBuckets) Up() error {
	if facades.Schema().HasTable("rate_limit_buckets") {
		return nil
	}

	return facades.Schema().Create("rate_limit_buckets", func(table schema.Blueprint) {
		table.String("scope", 32)
		table.String("client_key", 64)
		table.BigInteger("window_start")
		table.Integer("request_count").Default(0)
		table.Timestamp("updated_at")
		table.Unique("scope", "client_key")
		table.Index("updated_at")
	})
}

func (r *M20260721000002CreateRateLimitBuckets) Down() error {
	return facades.Schema().DropIfExists("rate_limit_buckets")
}
