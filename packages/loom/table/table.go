// Package table contains the declarative list (table) builder used by admin resources.
package table

import (
	"fmt"
	"strings"
	"time"

	"goravel/packages/loom/form"
)

// Row is a single database record.
type Row = map[string]any

// ColumnKind identifies how a cell is rendered.
type ColumnKind string

const (
	ColumnText     ColumnKind = "text"
	ColumnBadge    ColumnKind = "badge"
	ColumnBoolean  ColumnKind = "boolean"
	ColumnDateTime ColumnKind = "datetime"
	ColumnImage    ColumnKind = "image"
	ColumnFile     ColumnKind = "file"
)

// Column is a table column declared on a resource.
type Column struct {
	name       string
	kind       ColumnKind
	label      string
	sortable   bool
	searchable bool
	format     string
	colors     map[string]string
	formatter  func(value any, row Row) string
}

func Text(name string) *Column    { return &Column{name: name, kind: ColumnText} }
func Boolean(name string) *Column { return &Column{name: name, kind: ColumnBoolean} }

// Image shows a thumbnail of a FileUpload field (any storage: disk, base64 or binary).
func Image(name string) *Column { return &Column{name: name, kind: ColumnImage} }

// FileLink shows an "Abrir" link to a FileUpload field.
func FileLink(name string) *Column { return &Column{name: name, kind: ColumnFile} }
func DateTime(name string) *Column {
	return &Column{name: name, kind: ColumnDateTime, format: "02/01/2006 15:04"}
}

// Badge renders the value as a colored pill. colors maps value => color
// (gray, green, red, yellow, blue, indigo).
func Badge(name string, colors map[string]string) *Column {
	return &Column{name: name, kind: ColumnBadge, colors: colors}
}

func (c *Column) Label(label string) *Column { c.label = label; return c }
func (c *Column) Sortable() *Column          { c.sortable = true; return c }
func (c *Column) Searchable() *Column        { c.searchable = true; return c }

// Format sets the Go time layout for DateTime columns.
func (c *Column) Format(layout string) *Column { c.format = layout; return c }

// FormatUsing customises the displayed value.
func (c *Column) FormatUsing(fn func(value any, row Row) string) *Column {
	c.formatter = fn
	return c
}

func (c *Column) Name() string       { return c.name }
func (c *Column) Kind() string       { return string(c.kind) }
func (c *Column) IsSortable() bool   { return c.sortable }
func (c *Column) IsSearchable() bool { return c.searchable }
func (c *Column) LabelText() string {
	if c.label != "" {
		return c.label
	}
	return form.Headline(c.name)
}

// Display returns the textual value of the cell.
func (c *Column) Display(row Row) string {
	value := row[c.name]
	if c.formatter != nil {
		return c.formatter(value, row)
	}
	if c.kind == ColumnDateTime {
		if t, ok := value.(time.Time); ok {
			return t.Local().Format(c.format)
		}
	}
	return Stringify(value)
}

// Truthy reports whether the cell value is "true" (for Boolean columns).
func (c *Column) Truthy(row Row) bool {
	switch v := row[c.name].(type) {
	case bool:
		return v
	case nil:
		return false
	case []byte:
		return len(v) > 0 && string(v) != "0" && string(v) != "f" && string(v) != "false"
	default:
		s := strings.ToLower(Stringify(v))
		return s == "1" || s == "true" || s == "t"
	}
}

// Color returns the badge color for the cell.
func (c *Column) Color(row Row) string {
	if color, ok := c.colors[Stringify(row[c.name])]; ok {
		return color
	}
	return "gray"
}

// Filter is a select filter shown above the table.
type Filter struct {
	name    string
	label   string
	options []form.Option
}

// SelectFilter filters rows where column = selected value.
func SelectFilter(name string) *Filter { return &Filter{name: name} }

func (f *Filter) Label(label string) *Filter { f.label = label; return f }
func (f *Filter) Option(value, label string) *Filter {
	f.options = append(f.options, form.Option{Value: value, Label: label})
	return f
}

func (f *Filter) Name() string           { return f.name }
func (f *Filter) Options() []form.Option { return f.options }
func (f *Filter) LabelText() string {
	if f.label != "" {
		return f.label
	}
	return form.Headline(f.name)
}

// HasOption reports whether value is one of the declared options.
func (f *Filter) HasOption(value string) bool {
	for _, o := range f.options {
		if o.Value == value {
			return true
		}
	}
	return false
}

// Table is the list configuration of a resource.
type Table struct {
	columns     []*Column
	filters     []*Filter
	defaultSort string
	defaultDir  string
	perPage     int
	extra       []string
}

func New() *Table { return &Table{perPage: 10, defaultDir: "desc"} }

func (t *Table) Columns(columns ...*Column) *Table {
	t.columns = append(t.columns, columns...)
	return t
}

func (t *Table) Filters(filters ...*Filter) *Table {
	t.filters = append(t.filters, filters...)
	return t
}

// DefaultSort sets the initial ordering, e.g. DefaultSort("created_at", "desc").
func (t *Table) DefaultSort(column, direction string) *Table {
	t.defaultSort, t.defaultDir = column, direction
	return t
}

func (t *Table) PerPage(n int) *Table { t.perPage = n; return t }

// Select loads extra columns for FormatUsing when the list query is narrowed
// (resources with base64/binary FileUpload fields only load the declared columns).
func (t *Table) Select(columns ...string) *Table { t.extra = append(t.extra, columns...); return t }

// GetSelect returns the extra columns.
func (t *Table) GetSelect() []string { return t.extra }

func (t *Table) GetColumns() []*Column { return t.columns }
func (t *Table) GetFilters() []*Filter { return t.filters }
func (t *Table) GetPerPage() int       { return t.perPage }
func (t *Table) GetDefaultSort() (string, string) {
	return t.defaultSort, t.defaultDir
}

// Column finds a declared column by name.
func (t *Table) Column(name string) *Column {
	for _, c := range t.columns {
		if c.name == name {
			return c
		}
	}
	return nil
}

// Stringify converts a database value into a display string.
func Stringify(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	case time.Time:
		return val.Local().Format("02/01/2006 15:04")
	default:
		return fmt.Sprint(val)
	}
}
