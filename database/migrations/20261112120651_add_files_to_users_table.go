package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20261112120651AddFilesToUsersTable struct{}

// Signature The unique signature for the migration.
func (r *M20261112120651AddFilesToUsersTable) Signature() string {
	return "20261112120651_add_files_to_users_table"
}

// Up Run the migrations.
func (r *M20261112120651AddFilesToUsersTable) Up() error {
	return facades.Schema().Table("users", func(table schema.Blueprint) {
		// Loom FileUpload examples, one per storage mode:
		table.String("avatar").Nullable()            // disk: file in storage/app, path here
		table.Text("document").Nullable()            // base64: data URI in a text column
		table.Column("contract", "bytea").Nullable() // binary: raw bytes (PostgreSQL bytea)
	})
}

// Down Reverse the migrations.
func (r *M20261112120651AddFilesToUsersTable) Down() error {
	return facades.Schema().Table("users", func(table schema.Blueprint) {
		table.DropColumn("avatar", "document", "contract")
	})
}
