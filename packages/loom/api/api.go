// Package api declares the fields and access rules of Loom API resources.
//
//	func (Resource) Fields(s *api.Schema) {
//		s.Schema(
//			api.Field("brand").Required().Rules("max:100").Searchable().Sortable(),
//			api.Field("year").Integer().Filterable().Sortable(),
//			api.Field("created_at").DateTime().ReadOnly().Sortable(),
//		)
//	}
//
//	func (Resource) Access() api.Access {
//		return api.Access{Index: api.Public(), Show: api.Public(), Store: api.Can("cars:write")}
//	}
package api

import (
	"fmt"
	"strings"
)

// Kind is the JSON type of a field.
type Kind string

const (
	KindString   Kind = "string"
	KindInteger  Kind = "integer"
	KindNumber   Kind = "number"
	KindBoolean  Kind = "boolean"
	KindDate     Kind = "date"
	KindDateTime Kind = "datetime"
	KindPassword Kind = "password"
)

// FieldDef is a column exposed by an API resource.
type FieldDef struct {
	name       string
	kind       Kind
	required   bool
	unique     bool
	nullable   bool
	hidden     bool
	readOnly   bool
	searchable bool
	sortable   bool
	filterable bool
	rules      []string
}

// Field declares a column. It is a string unless a type modifier is used.
func Field(name string) *FieldDef { return &FieldDef{name: name, kind: KindString} }

func (f *FieldDef) Integer() *FieldDef  { f.kind = KindInteger; return f }
func (f *FieldDef) Number() *FieldDef   { f.kind = KindNumber; return f }
func (f *FieldDef) Boolean() *FieldDef  { f.kind = KindBoolean; return f }
func (f *FieldDef) Date() *FieldDef     { f.kind = KindDate; return f }
func (f *FieldDef) DateTime() *FieldDef { f.kind = KindDateTime; return f }

// Password hashes the value before saving and never returns it.
func (f *FieldDef) Password() *FieldDef { f.kind = KindPassword; f.hidden = true; return f }

// Required must be sent on create and cannot be emptied on update.
func (f *FieldDef) Required() *FieldDef { f.required = true; return f }

// Unique validates that no other row has the same value.
func (f *FieldDef) Unique() *FieldDef { f.unique = true; return f }

// Nullable accepts null (and "" for strings) and saves NULL.
func (f *FieldDef) Nullable() *FieldDef { f.nullable = true; return f }

// Hidden can be written but is never returned.
func (f *FieldDef) Hidden() *FieldDef { f.hidden = true; return f }

// ReadOnly is returned but cannot be written (ids, timestamps, computed columns).
func (f *FieldDef) ReadOnly() *FieldDef { f.readOnly = true; return f }

// Searchable includes the field in ?search=.
func (f *FieldDef) Searchable() *FieldDef { f.searchable = true; return f }

// Sortable allows ?sort=field and ?sort=-field.
func (f *FieldDef) Sortable() *FieldDef { f.sortable = true; return f }

// Filterable allows ?filter[field]=value (comma separated values match any).
func (f *FieldDef) Filterable() *FieldDef { f.filterable = true; return f }

// Rules appends Goravel validation rules, e.g. Rules("min:3", "max:100").
func (f *FieldDef) Rules(rules ...string) *FieldDef { f.rules = append(f.rules, rules...); return f }

func (f *FieldDef) Name() string       { return f.name }
func (f *FieldDef) Kind() Kind         { return f.kind }
func (f *FieldDef) IsRequired() bool   { return f.required }
func (f *FieldDef) IsNullable() bool   { return f.nullable }
func (f *FieldDef) IsHidden() bool     { return f.hidden }
func (f *FieldDef) IsReadOnly() bool   { return f.readOnly }
func (f *FieldDef) IsSearchable() bool { return f.searchable }
func (f *FieldDef) IsSortable() bool   { return f.sortable }
func (f *FieldDef) IsFilterable() bool { return f.filterable }

// ValidationRules builds the Goravel rules for a value that was sent.
// recordID is empty on create.
func (f *FieldDef) ValidationRules(table, recordID string) string {
	var rules []string
	if f.unique {
		rule := fmt.Sprintf("unique:%s,%s", table, f.name)
		if recordID != "" {
			rule += ",id," + recordID
		}
		rules = append(rules, rule)
	}
	rules = append(rules, f.rules...)
	return strings.Join(rules, "|")
}

// Schema is the list of fields of a resource.
type Schema struct{ fields []*FieldDef }

// NewSchema returns an empty schema.
func NewSchema() *Schema { return &Schema{} }

// Schema sets the fields.
func (s *Schema) Schema(fields ...*FieldDef) *Schema {
	s.fields = append(s.fields, fields...)
	return s
}

// Fields returns the declared fields.
func (s *Schema) Fields() []*FieldDef { return s.fields }

// Field returns the field with the given name, or nil.
func (s *Schema) Field(name string) *FieldDef {
	for _, f := range s.fields {
		if f.name == name {
			return f
		}
	}
	return nil
}

// Rule decides who can call an endpoint. The zero value requires a valid token.
type Rule struct {
	public  bool
	ability string
}

// Public allows requests without a token.
func Public() Rule { return Rule{public: true} }

// Authenticated requires any valid token.
func Authenticated() Rule { return Rule{} }

// Can requires a token with the ability (or "*").
func Can(ability string) Rule { return Rule{ability: ability} }

func (r Rule) IsPublic() bool  { return r.public }
func (r Rule) Ability() string { return r.ability }
func (r Rule) String() string {
	switch {
	case r.public:
		return "public"
	case r.ability != "":
		return "ability:" + r.ability
	default:
		return "authenticated"
	}
}

// Access holds the rule of each endpoint. Endpoints left out require a token.
type Access struct {
	Index   Rule // GET    /{slug}
	Show    Rule // GET    /{slug}/{id}
	Store   Rule // POST   /{slug}
	Update  Rule // PUT    /{slug}/{id}  and PATCH
	Destroy Rule // DELETE /{slug}/{id}
}

// ReadOnly makes Index and Show public and keeps the writes private.
func ReadOnly() Access { return Access{Index: Public(), Show: Public()} }
