package loom

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"github.com/goravel/framework/contracts/database/driver"
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/packages/loom/form"
	"goravel/packages/loom/heroicons"
)

// MakeResourceConfig tells `loom:make-resource` where to write and register resources.
type MakeResourceConfig struct {
	// Dir is where resource folders are created, e.g. "app/loom/admin".
	Dir string
	// Provider is the panel provider file where the resource is added to
	// Resources(...), e.g. "app/providers/admin_panel_provider.go". Empty = don't register.
	Provider string
	// Schema reads the table columns for --generate (usually facades.Schema).
	Schema func() schema.Schema
}

// MakeResourceCommand scaffolds a resource folder (like `php artisan make:filament-resource`):
//
//	./artisan loom:make-resource Car
//	./artisan loom:make-resource Car --generate          # fields and columns from the database
//	./artisan loom:make-resource Product --table=produtos --label=Produtos --singular=Produto
type MakeResourceCommand struct {
	cfg MakeResourceConfig
}

// NewMakeResourceCommand builds the command.
func NewMakeResourceCommand(cfg MakeResourceConfig) *MakeResourceCommand {
	if cfg.Dir == "" {
		cfg.Dir = "app/loom/admin"
	}
	return &MakeResourceCommand{cfg: cfg}
}

func (c *MakeResourceCommand) Signature() string { return "loom:make-resource" }
func (c *MakeResourceCommand) Description() string {
	return "Create a Loom resource (resource, form, table and page hooks)"
}

func (c *MakeResourceCommand) Extend() command.Extend {
	return command.Extend{
		Category:  "loom",
		ArgsUsage: " <name>",
		Flags: []command.Flag{
			&command.BoolFlag{Name: "generate", Aliases: []string{"g"}, Usage: "Generate form fields and table columns from the database table"},
			&command.StringFlag{Name: "table", Usage: "Database table (default: snake_case plural of the name)"},
			&command.StringFlag{Name: "slug", Usage: "URL slug (default: the table name with dashes)"},
			&command.StringFlag{Name: "label", Usage: "Plural label shown in the menu, e.g. Carros"},
			&command.StringFlag{Name: "singular", Usage: "Singular label, e.g. Carro"},
			&command.StringFlag{Name: "icon", Usage: "Heroicon name, e.g. truck", Value: "rectangle-stack"},
			&command.StringFlag{Name: "group", Usage: "Navigation group"},
			&command.BoolFlag{Name: "no-register", Usage: "Do not add the resource to the panel provider"},
			&command.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Overwrite existing files"},
		},
	}
}

// resourceSpec is everything the templates need.
type resourceSpec struct {
	Loom       string // import path of this package
	ClassPl    string // "BlogPosts", for comments
	ClassSg    string // "BlogPost"
	Package    string
	Import     string
	Table      string
	Slug       string
	Label      string
	Singular   string
	Icon       string
	Group      string
	PrimaryKey string // set only when it is not "id"
	NoStamps   bool
	Generated  bool
	Fields     []string
	Columns    []string
	Filters    []string
	SortColumn string
	SortDir    string
	HasForm    bool
	HasTable   bool
}

func (c *MakeResourceCommand) Handle(ctx console.Context) error {
	name := strings.TrimSpace(ctx.Argument(0))
	if name == "" {
		var err error
		name, err = ctx.Ask("Resource name (e.g. Car, BlogPost)", console.AskOption{Validate: func(s string) error {
			if len(splitWords(s)) == 0 {
				return errors.New("required")
			}
			return nil
		}})
		if err != nil {
			return err
		}
	}
	words := splitWords(name)
	if len(words) == 0 {
		ctx.Error("Invalid resource name.")
		return nil
	}

	module, err := goModule()
	if err != nil {
		ctx.Error(err.Error())
		return nil
	}

	pkg := strings.Join(words, "")
	if token.IsKeyword(pkg) || !token.IsIdentifier(pkg) {
		ctx.Error(fmt.Sprintf("%q cannot be used as a Go package name; pick another name.", pkg))
		return nil
	}
	plural := append(append([]string{}, words[:len(words)-1]...), pluralize(words[len(words)-1]))

	loomPath := reflect.TypeOf(Panel{}).PkgPath()
	spec := resourceSpec{
		Loom:     loomPath,
		Package:  pkg,
		Import:   module + "/" + filepath.ToSlash(filepath.Join(c.cfg.Dir, pkg)),
		Table:    strings.Join(plural, "_"),
		Label:    titleWords(plural),
		Singular: titleWords(words),
		ClassPl:  strings.ReplaceAll(titleWords(plural), " ", ""),
		ClassSg:  strings.ReplaceAll(titleWords(words), " ", ""),
		Icon:     ctx.Option("icon"),
		Group:    ctx.Option("group"),
	}
	if t := ctx.Option("table"); t != "" {
		spec.Table = t
	}
	spec.Slug = strings.ReplaceAll(spec.Table, "_", "-")
	if s := ctx.Option("slug"); s != "" {
		spec.Slug = strings.Trim(s, "/")
	}
	if l := ctx.Option("label"); l != "" {
		spec.Label = l
	}
	if l := ctx.Option("singular"); l != "" {
		spec.Singular = l
	}
	if !heroicons.Exists(spec.Icon) {
		ctx.Error(fmt.Sprintf("Unknown heroicon %q (see https://heroicons.com).", spec.Icon))
		return nil
	}

	if ctx.OptionBool("generate") {
		if c.cfg.Schema == nil {
			ctx.Error("--generate needs MakeResourceConfig.Schema (e.g. facades.Schema).")
			return nil
		}
		if err := spec.generate(c.cfg.Schema()); err != nil {
			ctx.Error(err.Error())
			return nil
		}
	} else {
		spec.Columns = []string{`table.Text("id").Label("ID").Sortable(),`}
		spec.SortColumn, spec.SortDir = "id", "desc"
	}
	spec.HasForm = len(spec.Fields) > 0
	spec.HasTable = len(spec.Columns) > 0

	dir := filepath.Join(c.cfg.Dir, pkg)
	files := map[string]*template.Template{
		"resource.go": resourceTpl,
		"form.go":     formTpl,
		"table.go":    tableTpl,
		"pages.go":    pagesTpl,
	}
	if !ctx.OptionBool("force") {
		for f := range files {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				ctx.Error(fmt.Sprintf("%s already exists (use --force to overwrite).", filepath.Join(dir, f)))
				return nil
			}
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range []string{"resource.go", "form.go", "table.go", "pages.go"} {
		var buf bytes.Buffer
		if err := files[f].Execute(&buf, spec); err != nil {
			return err
		}
		src, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("loom: generated %s is invalid: %w\n%s", f, err, buf.String())
		}
		if err := os.WriteFile(filepath.Join(dir, f), src, 0o644); err != nil {
			return err
		}
		ctx.Info("Created " + filepath.Join(dir, f))
	}

	if c.cfg.Provider != "" && !ctx.OptionBool("no-register") {
		added, err := registerResource(c.cfg.Provider, pkg, spec.Import)
		switch {
		case err != nil:
			ctx.Warning("Could not register it automatically (" + err.Error() + "). Add &" + pkg + ".Resource{} to Resources(...) in " + c.cfg.Provider)
		case added:
			ctx.Info("Registered in " + c.cfg.Provider)
		}
	}

	ctx.Success(fmt.Sprintf("Resource %s created: /%s. Restart the app to see it.", spec.Label, spec.Slug))
	if !ctx.OptionBool("generate") {
		ctx.Line("Tip: use --generate to build the form and table from the " + spec.Table + " columns.")
	}
	return nil
}

type colKind int

const (
	kindString colKind = iota
	kindText
	kindBool
	kindInt
	kindDecimal
	kindDate
	kindDateTime
	kindBinary
	kindOther
)

var lengthRe = regexp.MustCompile(`\((\d+)\)`)

func classify(c driver.Column) colKind {
	tn := strings.ToLower(c.TypeName)
	full := strings.ToLower(c.Type)
	switch {
	case tn == "bool" || tn == "boolean" || full == "tinyint(1)":
		return kindBool
	case tn == "bytea" || strings.Contains(tn, "blob") || strings.Contains(tn, "binary"):
		return kindBinary
	case strings.HasPrefix(tn, "timestamp") || tn == "datetime" || tn == "timestamptz":
		return kindDateTime
	case tn == "date":
		return kindDate
	case strings.HasPrefix(tn, "int") || strings.HasSuffix(tn, "int") || strings.Contains(tn, "serial") ||
		tn == "integer":
		return kindInt
	case tn == "numeric" || tn == "decimal" || strings.HasPrefix(tn, "float") || tn == "real" ||
		strings.HasPrefix(tn, "double") || tn == "money":
		return kindDecimal
	case strings.Contains(tn, "text") || strings.HasPrefix(tn, "json") || tn == "xml":
		return kindText
	case strings.Contains(tn, "char") || tn == "string" || tn == "citext":
		return kindString
	default:
		return kindOther // uuid, time, inet, enum...
	}
}

func (s *resourceSpec) generate(sch schema.Schema) error {
	if !sch.HasTable(s.Table) {
		return fmt.Errorf("table %q not found: run the migration first or pass --table", s.Table)
	}
	cols, err := sch.GetColumns(s.Table)
	if err != nil {
		return err
	}
	indexes, _ := sch.GetIndexes(s.Table)

	pk := "id"
	unique := map[string]bool{}
	for _, ix := range indexes {
		if ix.Primary && len(ix.Columns) == 1 {
			pk = ix.Columns[0]
		} else if ix.Unique && len(ix.Columns) == 1 {
			unique[ix.Columns[0]] = true
		}
	}
	if pk != "id" {
		s.PrimaryKey = pk
	}
	s.Generated = true

	has := map[string]bool{}
	for _, c := range cols {
		has[c.Name] = true
	}
	s.NoStamps = !has["created_at"] || !has["updated_at"]

	s.Columns = append(s.Columns, fmt.Sprintf(`table.Text(%q).Label("ID").Sortable(),`, pk))
	for _, c := range cols {
		name := c.Name
		kind := classify(c)
		label := form.Headline(name)
		if strings.HasSuffix(name, "_id") {
			label = form.Headline(strings.TrimSuffix(name, "_id"))
		}
		system := name == pk || name == "created_at" || name == "updated_at" || name == "deleted_at" || name == "remember_token"
		secret := strings.Contains(name, "password") || strings.Contains(name, "token") || strings.Contains(name, "secret")
		required := !c.Nullable && c.Default == "" && !c.Autoincrement

		// Form field.
		if !system {
			var f string
			switch {
			case strings.Contains(name, "password"):
				f = fmt.Sprintf(`form.Password(%q).Label(%q)`, name, label)
				if required {
					f += `.Required().Rules("min:8")`
				}
				required = false
			case kind == kindBool:
				f = fmt.Sprintf(`form.Toggle(%q).Label(%q)`, name, label)
				if d := strings.ToLower(strings.Trim(c.Default, "'")); d == "true" || d == "1" {
					f += `.Default("true")`
				}
				required = false
			case kind == kindInt || kind == kindDecimal:
				f = fmt.Sprintf(`form.Number(%q).Label(%q)`, name, label)
			case kind == kindDate:
				f = fmt.Sprintf(`form.Date(%q).Label(%q)`, name, label)
			case kind == kindDateTime:
				f = fmt.Sprintf(`form.Date(%q).Label(%q) // %s column: the form edits the date only`, name, label, c.TypeName)
			case kind == kindText:
				f = fmt.Sprintf(`form.Textarea(%q).Label(%q)`, name, label)
			case kind == kindBinary:
				f = fmt.Sprintf(`form.FileUpload(%q).Label(%q).StoreAsBinary()`, name, label)
			case strings.Contains(name, "email"):
				f = fmt.Sprintf(`form.Email(%q).Label(%q)`, name, label)
			default:
				f = fmt.Sprintf(`form.Text(%q).Label(%q)`, name, label)
			}
			if required {
				f = appendCall(f, ".Required()")
			} else if c.Nullable && (kind == kindString || kind == kindText || kind == kindOther) {
				f = appendCall(f, ".Nullable()")
			}
			if unique[name] {
				f = appendCall(f, ".Unique()")
			}
			if kind == kindString {
				if m := lengthRe.FindStringSubmatch(c.Type); m != nil {
					f = appendCall(f, fmt.Sprintf(`.Rules("max:%s")`, m[1]))
				}
			}
			s.Fields = append(s.Fields, appendCall(f, ","))
		}

		// Table column.
		if name == pk || secret || name == "updated_at" || name == "deleted_at" {
			continue
		}
		switch kind {
		case kindBool:
			s.Columns = append(s.Columns, fmt.Sprintf(`table.Boolean(%q).Label(%q),`, name, label))
			s.Filters = append(s.Filters, fmt.Sprintf(`table.SelectFilter(%q).Label(%q).Option("true", "Sim").Option("false", "Não"),`, name, label))
		case kindDate, kindDateTime:
			s.Columns = append(s.Columns, fmt.Sprintf(`table.DateTime(%q).Label(%q).Sortable(),`, name, label))
		case kindInt, kindDecimal:
			s.Columns = append(s.Columns, fmt.Sprintf(`table.Text(%q).Label(%q).Sortable(),`, name, label))
		case kindString:
			s.Columns = append(s.Columns, fmt.Sprintf(`table.Text(%q).Label(%q).Sortable().Searchable(),`, name, label))
		case kindBinary:
			s.Columns = append(s.Columns, fmt.Sprintf(`table.FileLink(%q).Label(%q),`, name, label))
		case kindOther:
			s.Columns = append(s.Columns, fmt.Sprintf(`table.Text(%q).Label(%q),`, name, label))
		}
	}
	// created_at goes last, like Filament.
	for i, col := range s.Columns {
		if strings.HasPrefix(col, `table.DateTime("created_at")`) {
			s.Columns = append(append(s.Columns[:i:i], s.Columns[i+1:]...), col)
			break
		}
	}

	s.SortColumn, s.SortDir = pk, "desc"
	if has["created_at"] {
		s.SortColumn = "created_at"
	}
	return nil
}

// appendCall adds a call/suffix to a field expression, before any trailing // comment.
func appendCall(expr, call string) string {
	if i := strings.Index(expr, " //"); i >= 0 {
		return expr[:i] + call + expr[i:]
	}
	return expr + call
}

// splitWords turns "BlogPost", "blog_post", "blog-post" or "blog post" into ["blog", "post"].
func splitWords(s string) []string {
	var words []string
	var cur []rune
	runes := []rune(strings.TrimSpace(s))
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == ' ' || r == '/' || r == '.':
			flush()
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			// drop
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(runes[i-1]) ||
			(i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1]))):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	// Package names must be ASCII: strip accents ("Usuário" => "usuario").
	for i, w := range words {
		words[i] = accentFold(w)
	}
	return words
}

var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "è", "e", "ë", "e",
	"í", "i", "î", "i", "ì", "i", "ï", "i",
	"ó", "o", "ô", "o", "õ", "o", "ò", "o", "ö", "o",
	"ú", "u", "û", "u", "ù", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

func accentFold(s string) string { return accentFolder.Replace(s) }

// pluralize applies simple English rules (pass --table for other languages).
func pluralize(w string) string {
	switch {
	case strings.HasSuffix(w, "s") || strings.HasSuffix(w, "x") || strings.HasSuffix(w, "z") ||
		strings.HasSuffix(w, "ch") || strings.HasSuffix(w, "sh"):
		return w + "es"
	case strings.HasSuffix(w, "y") && len(w) > 1 && !strings.ContainsRune("aeiou", rune(w[len(w)-2])):
		return w[:len(w)-1] + "ies"
	default:
		return w + "s"
	}
}

func titleWords(words []string) string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(out, " ")
}

func goModule() (string, error) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return "", errors.New("go.mod not found: run the command from the project root")
	}
	for _, line := range strings.Split(string(b), "\n") {
		if m, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(m), nil
		}
	}
	return "", errors.New("go.mod has no module line")
}

// registerResource adds `&pkg.Resource{}` to the Resources(...) call of the
// provider file and imports the package. It reports false when already there.
func registerResource(file, pkg, importPath string) (bool, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return false, err
	}

	alias := pkg
	imported := false
	used := map[string]bool{}
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := filepath.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if p == importPath {
			imported, alias = true, name
		}
		used[name] = true
	}
	if !imported && used[alias] {
		alias = pkg + "resource" // e.g. a resource named "table" or "widgets"
	}

	var call *ast.CallExpr
	ast.Inspect(f, func(n ast.Node) bool {
		if ce, ok := n.(*ast.CallExpr); ok && call == nil {
			if sel, ok := ce.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Resources" {
				call = ce
			}
		}
		return call == nil
	})
	if call == nil {
		return false, errors.New("no Resources(...) call found")
	}
	entry := "&" + alias + ".Resource{}"
	for _, arg := range call.Args {
		if string(src[fset.Position(arg.Pos()).Offset:fset.Position(arg.End()).Offset]) == entry {
			return false, nil
		}
	}

	// Insert from the end of the file backwards so earlier offsets stay valid.
	out := string(src)
	rp := fset.Position(call.Rparen).Offset
	if nl := strings.LastIndex(out[:rp], "\n"); nl >= 0 && strings.TrimSpace(out[nl:rp]) == "" {
		out = out[:nl+1] + entry + ",\n" + out[nl+1:] // multi-line call: new line before ")"
	} else {
		out = out[:rp] + "\n" + entry + ",\n" + out[rp:]
	}

	if !imported {
		spec := `"` + importPath + `"`
		if alias != pkg {
			spec = alias + " " + spec
		}
		var decl *ast.GenDecl
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
				decl = g
				break
			}
		}
		switch {
		case decl == nil:
			return false, errors.New("no import block found")
		case decl.Rparen.IsValid():
			// After the last import of the block's last group; gofmt sorts it.
			last := decl.Specs[len(decl.Specs)-1]
			at := fset.Position(last.End()).Offset
			out = out[:at] + "\n\t" + spec + out[at:]
		default:
			at := fset.Position(decl.End()).Offset
			out = out[:at] + "\nimport " + spec + out[at:]
		}
	}

	formatted, err := format.Source([]byte(out))
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(file, formatted, 0o644)
}

var resourceTpl = template.Must(template.New("resource").Parse(`// Package {{.Package}} is the admin panel resource for the {{.Table}} table.
//
//	resource.go  identity: slug, table, labels, icon, navigation
//	form.go      create/edit form schema
//	table.go     list columns, filters and sorting
//	pages.go     list/create/edit page hooks
package {{.Package}}

import "{{.Loom}}"

// Resource manages the {{.Table}} table in the admin panel.
type Resource struct {
	loom.BaseResource
}

func (Resource) Slug() string          { return {{printf "%q" .Slug}} }
func (Resource) TableName() string     { return {{printf "%q" .Table}} }
func (Resource) Label() string         { return {{printf "%q" .Label}} }
func (Resource) SingularLabel() string { return {{printf "%q" .Singular}} }
func (Resource) Icon() string          { return {{printf "%q" .Icon}} }

func (Resource) NavigationGroup() string { return {{printf "%q" .Group}} }
func (Resource) NavigationSort() int     { return 0 }
{{- if .PrimaryKey}}

func (Resource) PrimaryKey() string { return {{printf "%q" .PrimaryKey}} }
{{- end}}
{{- if .NoStamps}}

// The table has no created_at/updated_at pair, so they are not filled automatically.
func (Resource) Timestamps() bool { return false }
{{- end}}
`))

var formTpl = template.Must(template.New("form").Parse(`package {{.Package}}

import "{{.Loom}}/form"

// Form defines the create/edit form.
// Fields: Text, Email, Password, Number, Textarea, Select, Toggle, Date, FileUpload.
func (Resource) Form(f *form.Form) {
	f.Schema(
{{- range .Fields}}
		{{.}}
{{- end}}
{{- if not .HasForm}}
		// form.Text("name").Label("Nome").Required().Rules("max:255"),
		// form.Email("email").Label("E-mail").Required().Unique(),
		// form.Number("price").Label("Preço").Required(),
		// form.Select("status").Label("Status").Option("draft", "Rascunho").Option("published", "Publicado"),
		// form.Toggle("is_active").Label("Ativo").Default("true"),
		// form.FileUpload("photo").Label("Foto").Image().Directory({{printf "%q" .Table}}),
{{- end}}
	)
}
`))

var tableTpl = template.Must(template.New("table").Parse(`package {{.Package}}

import "{{.Loom}}/table"

// Table defines the list page.
// Columns: Text, Boolean, DateTime, Badge, Image, FileLink.
func (Resource) Table(t *table.Table) {
	t.Columns(
{{- range .Columns}}
		{{.}}
{{- end}}
{{- if not .Generated}}
		// table.Text("name").Label("Nome").Sortable().Searchable(),
		// table.Boolean("is_active").Label("Ativo"),
		// table.DateTime("created_at").Label("Criado em").Sortable(),
{{- end}}
	).Filters(
{{- range .Filters}}
		{{.}}
{{- end}}
{{- if not .Filters}}
		// table.SelectFilter("status").Label("Status").Option("draft", "Rascunho"),
{{- end}}
	).DefaultSort({{printf "%q" .SortColumn}}, {{printf "%q" .SortDir}})
}
`))

var pagesTpl = template.Must(template.New("pages").Parse(`package {{.Package}}

// Page hooks: the list, create and edit pages of this resource, like the
// List{{.ClassPl}}, Create{{.ClassSg}} and Edit{{.ClassSg}} classes of Filament.
// Every method is optional: uncomment what you need and add the imports:
//
//	contractsdb "github.com/goravel/framework/contracts/database/db"
//	"{{.Loom}}"

// ModifyListQuery changes the query of the list (scopes, joins, hidden rows).
//
// func (Resource) ModifyListQuery(ctx loom.Context, q contractsdb.Query) contractsdb.Query {
// 	return q.Where("is_active", true)
// }

// BeforeCreate runs after validation, before the INSERT. Change data or stop with loom.Halt.
//
// func (Resource) BeforeCreate(ctx loom.Context, data map[string]any) error {
// 	data["created_by"] = ctx.User()["id"]
// 	return nil
// }

// AfterCreate runs after the INSERT (send an e-mail, write a log...).
//
// func (Resource) AfterCreate(ctx loom.Context, data map[string]any) error {
// 	return nil
// }

// BeforeSave runs after validation, before the UPDATE.
//
// func (Resource) BeforeSave(ctx loom.Context, id string, data map[string]any) error {
// 	if data["status"] == "published" && ctx.User()["role"] != "admin" {
// 		return loom.Halt("Só administradores podem publicar.")
// 	}
// 	return nil
// }

// AfterSave runs after the UPDATE.
//
// func (Resource) AfterSave(ctx loom.Context, id string, data map[string]any) error {
// 	return nil
// }

// BeforeDelete runs before a single or bulk delete; loom.Halt cancels it.
//
// func (Resource) BeforeDelete(ctx loom.Context, ids []any) error {
// 	return nil
// }
`))
