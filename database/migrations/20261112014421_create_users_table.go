package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

type M20261112014421CreateUsersTable struct{}

// Signature The unique signature for the migration.
func (r *M20261112014421CreateUsersTable) Signature() string {
	return "20261112014421_create_users_table"
}

// Up Run the migrations.
func (r *M20261112014421CreateUsersTable) Up() error {
	if !facades.Schema().HasTable("users") {
		return facades.Schema().Create("users", func(table schema.Blueprint) {
			table.ID()
			table.String("name")
			table.String("email")
			table.Unique("email")
			table.String("password")
			table.String("role").Default("user")
			table.Boolean("is_active").Default(true)
			table.TimestampsTz()
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20261112014421CreateUsersTable) Down() error {
	return facades.Schema().DropIfExists("users")
}
