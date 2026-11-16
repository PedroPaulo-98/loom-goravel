package bootstrap

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/database/migrations"
)

func Migrations() []schema.Migration {
	return []schema.Migration{
		&migrations.M20210101000001CreateJobsTable{},
		&migrations.M20261112014421CreateUsersTable{},
		&migrations.M20261112021827CreatePasswordResetTokensTable{},
		&migrations.M20261112120651AddFilesToUsersTable{},
	}
}
