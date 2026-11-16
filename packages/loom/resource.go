package loom

import (
	"goravel/packages/loom/form"
	"goravel/packages/loom/table"
)

// Resource describes a database table managed by the panel (like a Filament Resource).
//
// Embed BaseResource to get sensible defaults and only implement what you need:
//
//	type UserResource struct{ loom.BaseResource }
//	func (UserResource) Slug() string      { return "users" }
//	func (UserResource) TableName() string { return "users" }
//	func (UserResource) Form(f *form.Form)   { ... }
//	func (UserResource) Table(t *table.Table) { ... }
type Resource interface {
	// Slug is the URL segment: /admin/{slug}.
	Slug() string
	// TableName is the database table.
	TableName() string
	// Label is the plural name shown in navigation and titles.
	Label() string
	// SingularLabel is used in buttons and messages ("New user").
	SingularLabel() string
	// Icon is a Heroicon name for the sidebar, e.g. "users" or "heroicon-s-users".
	// See https://heroicons.com. Optional (defaults to "rectangle-stack").
	Icon() string
	// PrimaryKey column, usually "id".
	PrimaryKey() string
	// Timestamps reports whether created_at/updated_at are managed automatically.
	Timestamps() bool
	Form(f *form.Form)
	Table(t *table.Table)
}

// BaseResource provides defaults for optional Resource methods.
type BaseResource struct{}

func (BaseResource) Label() string         { return "" }
func (BaseResource) SingularLabel() string { return "" }
func (BaseResource) Icon() string          { return "" }
func (BaseResource) PrimaryKey() string    { return "id" }
func (BaseResource) Timestamps() bool      { return true }

// NavigationGroup places the resource under a sidebar group ("" = no group).
func (BaseResource) NavigationGroup() string { return "" }

// NavigationSort orders items inside their group (lower first, then by label).
func (BaseResource) NavigationSort() int { return 0 }

// resourceEntry caches the built form/table of a resource.
type resourceEntry struct {
	Resource
	form  *form.Form
	table *table.Table
}

func newEntry(r Resource) *resourceEntry {
	f := form.New()
	r.Form(f)
	t := table.New()
	r.Table(t)
	return &resourceEntry{Resource: r, form: f, table: t}
}

func (e *resourceEntry) PluralLabel() string {
	if l := e.Resource.Label(); l != "" {
		return l
	}
	return form.Headline(e.Slug())
}

func (e *resourceEntry) Singular() string {
	if l := e.SingularLabel(); l != "" {
		return l
	}
	return e.PluralLabel()
}

func (e *resourceEntry) IconName() string {
	if i := e.Icon(); i != "" {
		return i
	}
	return "rectangle-stack"
}
