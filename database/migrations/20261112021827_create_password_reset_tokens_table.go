package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20261112021827CreatePasswordResetTokensTable struct{}

// Signature The unique signature for the migration.
func (r *M20261112021827CreatePasswordResetTokensTable) Signature() string {
	return "20261112021827_create_password_reset_tokens_table"
}

// Up Run the migrations.
func (r *M20261112021827CreatePasswordResetTokensTable) Up() error {
	if !facades.Schema().HasTable("password_reset_tokens") {
		return facades.Schema().Create("password_reset_tokens", func(table schema.Blueprint) {
			table.String("email")
			table.String("token") // SHA-256 of the token sent by e-mail
			table.TimestampTz("created_at")
			table.Index("email")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20261112021827CreatePasswordResetTokensTable) Down() error {
	return facades.Schema().DropIfExists("password_reset_tokens")
}
