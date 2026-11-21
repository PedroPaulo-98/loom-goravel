package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20261116120000CreateApiTokensTable struct{}

// Signature The unique signature for the migration.
func (r *M20261116120000CreateApiTokensTable) Signature() string {
	return "20261116120000_create_api_tokens_table"
}

// Up Run the migrations.
func (r *M20261116120000CreateApiTokensTable) Up() error {
	if !facades.Schema().HasTable("api_tokens") {
		return facades.Schema().Create("api_tokens", func(table schema.Blueprint) {
			table.ID()
			table.BigInteger("user_id")
			table.String("name")
			table.String("token", 64) // SHA-256 of the token
			table.Text("abilities").Nullable()
			table.TimestampTz("last_used_at").Nullable()
			table.TimestampTz("expires_at").Nullable()
			table.TimestampsTz()
			table.Unique("token")
			table.Index("user_id")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20261116120000CreateApiTokensTable) Down() error {
	return facades.Schema().DropIfExists("api_tokens")
}
