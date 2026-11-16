// Package form contains the declarative form builder used by admin resources.
package form

import (
	"fmt"
	"sort"
	"strings"
)

// Kind identifies how a field is rendered and how its value is handled.
type Kind string

const (
	KindText     Kind = "text"
	KindEmail    Kind = "email"
	KindPassword Kind = "password"
	KindNumber   Kind = "number"
	KindTextarea Kind = "textarea"
	KindSelect   Kind = "select"
	KindToggle   Kind = "toggle"
	KindDate     Kind = "date"
	KindFile     Kind = "file"
)

// Option is a select option. Options keep their declaration order.
type Option struct {
	Value string
	Label string
}

// Field is a single form input declared on a resource.
type Field struct {
	name         string
	kind         Kind
	label        string
	placeholder  string
	help         string
	defaultValue string
	required     bool
	unique       bool
	nullable     bool
	onlyCreate   bool
	rules        []string
	options      []Option
	span         int
	upload       *Upload
}

func newField(kind Kind, name string) *Field {
	return &Field{name: name, kind: kind, span: 1}
}

func Text(name string) *Field     { return newField(KindText, name) }
func Email(name string) *Field    { return newField(KindEmail, name) }
func Password(name string) *Field { return newField(KindPassword, name) }
func Number(name string) *Field   { return newField(KindNumber, name) }
func Textarea(name string) *Field { return newField(KindTextarea, name).ColumnSpanFull() }
func Select(name string) *Field   { return newField(KindSelect, name) }
func Toggle(name string) *Field   { return newField(KindToggle, name) }
func Date(name string) *Field     { return newField(KindDate, name) }

func (f *Field) Label(label string) *Field             { f.label = label; return f }
func (f *Field) Placeholder(placeholder string) *Field { f.placeholder = placeholder; return f }
func (f *Field) Help(help string) *Field               { f.help = help; return f }
func (f *Field) Default(value string) *Field           { f.defaultValue = value; return f }
func (f *Field) Required() *Field                      { f.required = true; return f }

// Unique validates that no other row of the resource table has the same value.
func (f *Field) Unique() *Field { f.unique = true; return f }

// Nullable saves an empty value as NULL instead of "" (needed for uuid,
// json and other non-text columns, and nicer for optional text columns).
func (f *Field) Nullable() *Field { f.nullable = true; return f }

// OnlyOnCreate hides the field on the edit form.
func (f *Field) OnlyOnCreate() *Field { f.onlyCreate = true; return f }

// Rules appends raw Goravel validation rules, e.g. Rules("min:3", "max:255").
func (f *Field) Rules(rules ...string) *Field { f.rules = append(f.rules, rules...); return f }

// ColumnSpanFull makes the field take the whole row of the form grid.
func (f *Field) ColumnSpanFull() *Field { f.span = 2; return f }

// Options sets select options from value => label pairs (sorted by label).
func (f *Field) Options(options map[string]string) *Field {
	f.options = f.options[:0]
	for value, label := range options {
		f.options = append(f.options, Option{Value: value, Label: label})
	}
	sort.Slice(f.options, func(i, j int) bool { return f.options[i].Label < f.options[j].Label })
	return f
}

// Option appends a single select option, keeping declaration order.
func (f *Field) Option(value, label string) *Field {
	f.options = append(f.options, Option{Value: value, Label: label})
	return f
}

func (f *Field) Name() string            { return f.name }
func (f *Field) Kind() Kind              { return f.kind }
func (f *Field) PlaceholderText() string { return f.placeholder }
func (f *Field) HelpText() string        { return f.help }
func (f *Field) DefaultValue() string    { return f.defaultValue }
func (f *Field) IsRequired() bool        { return f.required }
func (f *Field) IsOnlyOnCreate() bool    { return f.onlyCreate }
func (f *Field) IsNullable() bool        { return f.nullable }
func (f *Field) SelectOptions() []Option { return f.options }
func (f *Field) IsFullWidth() bool       { return f.span == 2 }
func (f *Field) Is(kind string) bool     { return string(f.kind) == kind }
func (f *Field) LabelText() string {
	if f.label != "" {
		return f.label
	}
	return Headline(f.name)
}

// ValidationRules builds the Goravel rule string for this field.
// table and recordID are used for the unique rule (recordID is empty on create).
// On edit, an empty password means "keep the current one", so it is not required.
func (f *Field) ValidationRules(table, recordID string) string {
	var rules []string
	isEdit := recordID != ""

	switch {
	case f.required && f.kind == KindPassword && isEdit:
		// optional on edit
	case f.kind == KindFile:
		return "" // files are validated by the upload handler, not by Goravel rules
	case f.required && f.kind != KindToggle:
		rules = append(rules, "required")
	}

	switch f.kind {
	case KindEmail:
		rules = append(rules, "email")
	case KindNumber:
		rules = append(rules, "numeric")
	case KindDate:
		rules = append(rules, "date")
	case KindSelect:
		if len(f.options) > 0 {
			values := make([]string, 0, len(f.options))
			for _, o := range f.options {
				values = append(values, o.Value)
			}
			rules = append(rules, "in:"+strings.Join(values, ","))
		}
	}

	if f.unique {
		rule := fmt.Sprintf("unique:%s,%s", table, f.name)
		if isEdit {
			rule += ",id," + recordID
		}
		rules = append(rules, rule)
	}

	rules = append(rules, f.rules...)
	return strings.Join(rules, "|")
}

// Form is the collection of fields of a resource.
type Form struct {
	fields []*Field
}

func New() *Form { return &Form{} }

// Schema sets the form fields.
func (f *Form) Schema(fields ...*Field) *Form {
	f.fields = append(f.fields, fields...)
	return f
}

// Fields returns the fields visible for the given operation.
func (f *Form) Fields(isEdit bool) []*Field {
	out := make([]*Field, 0, len(f.fields))
	for _, field := range f.fields {
		if isEdit && field.onlyCreate {
			continue
		}
		out = append(out, field)
	}
	return out
}

// Headline converts snake_case names into "Snake case" labels.
func Headline(name string) string {
	s := strings.ReplaceAll(name, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
