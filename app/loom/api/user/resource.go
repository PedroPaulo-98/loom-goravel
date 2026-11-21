// Package user is the API resource for the users table: /api/v1/users.
package user

import (
	"goravel/packages/loom"
	"goravel/packages/loom/api"
)

// Resource exposes the users table as JSON.
type Resource struct {
	loom.BaseAPIResource
}

func (Resource) Slug() string      { return "users" }
func (Resource) TableName() string { return "users" }

// Access: every endpoint needs a token with the ability.
func (Resource) Access() api.Access {
	return api.Access{
		Index:   api.Can("users:read"),
		Show:    api.Can("users:read"),
		Store:   api.Can("users:write"),
		Update:  api.Can("users:write"),
		Destroy: api.Can("users:delete"),
	}
}

// Fields: what the API accepts and returns. Files are not exposed.
func (Resource) Fields(s *api.Schema) {
	s.Schema(
		api.Field("name").Required().Rules("max:255").Searchable().Sortable(),
		api.Field("email").Required().Unique().Rules("email").Searchable().Sortable(),
		api.Field("password").Password().Required().Rules("min:8"),
		api.Field("role").Rules("in:admin,user").Filterable(),
		api.Field("is_active").Boolean().Filterable(),
		api.Field("created_at").DateTime().ReadOnly().Sortable(),
		api.Field("updated_at").DateTime().ReadOnly(),
	)
}
