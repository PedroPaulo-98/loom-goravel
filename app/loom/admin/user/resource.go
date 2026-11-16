// Package user is the admin panel resource for the users table.
//
//	resource.go  identity: slug, table, labels, icon
//	form.go      create/edit form schema
//	table.go     list columns, filters and sorting
package user

import "goravel/packages/loom"

// Resource manages the users table in the admin panel.
type Resource struct {
	loom.BaseResource
}

func (Resource) Slug() string          { return "users" }
func (Resource) TableName() string     { return "users" }
func (Resource) Label() string         { return "Usuários" }
func (Resource) SingularLabel() string { return "Usuário" }
func (Resource) Icon() string          { return "users" }

func (Resource) NavigationGroup() string { return "Administração" }
