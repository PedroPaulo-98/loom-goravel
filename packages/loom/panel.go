// Package loom is an admin panel builder for Goravel, inspired by Laravel Filament.
//
// A panel is configured with a fluent API, like Filament's PanelProvider:
//
//	loom.NewPanel().
//		ID("admin").
//		Path("/admin").
//		BrandName("My App").
//		DefaultThemeMode(loom.ThemeDark).
//		Colors(map[string]color.Palette{"primary": color.Sky}).
//		NavigationGroups("Gestão", "Sistema").
//		Registration().
//		PasswordReset().
//		Resources(&user.Resource{}).
//		Widgets(&widgets.UsersStats{}).
//		Register(services)
package loom

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	contractsdb "github.com/goravel/framework/contracts/database/db"
	"github.com/goravel/framework/contracts/filesystem"
	"github.com/goravel/framework/contracts/hash"
	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/mail"
	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/contracts/validation"
	sessionmiddleware "github.com/goravel/framework/session/middleware"

	"goravel/packages/loom/color"
	"goravel/packages/loom/form"
	"goravel/packages/loom/heroicons"
	"goravel/packages/loom/table"
)

// Services are the Goravel services the panel depends on.
// Pass them from your app facades (facades.Route(), facades.DB(), ...).
type Services struct {
	Route      route.Route
	DB         contractsdb.DB
	Validation validation.Validation
	Hash       hash.Hash
	// Mail is required only when PasswordReset() is enabled.
	Mail mail.Mail
	// Storage is required only by FileUpload fields stored on disk.
	Storage filesystem.Storage
}

// ThemeMode is the initial color scheme of the panel.
type ThemeMode string

const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

// Plugin extends a panel (like Filament plugins). Register runs before the
// panel is validated, so a plugin can add resources, pages, widgets and routes.
type Plugin interface {
	ID() string
	Register(panel *Panel)
}

// Panel is an admin panel mounted at a path.
type Panel struct {
	id               string
	path             string
	brand            string
	baseURL          string
	themeMode        ThemeMode
	maxContentWidth  string
	colors           map[string]color.Palette
	navigationGroups []string
	sidebarCollapse  bool

	resources []*resourceEntry
	bySlug    map[string]*resourceEntry
	pages     []Page
	widgets   []Widget
	plugins   []Plugin
	routes    []func(r route.Router)

	middleware     []contractshttp.Middleware
	authMiddleware []contractshttp.Middleware

	authTable      string
	emailColumn    string
	passwordColumn string
	nameColumn     string
	canAccess      func(user table.Row) bool
	passwordRules  string

	registration     bool
	newUserDefaults  map[string]any
	passwordReset    bool
	resetTable       string
	resetExpiry      time.Duration
	resetEmailSender func(to, link string) error

	svc    Services
	render *renderer
}

// NewPanel creates a panel with Filament's defaults: id "admin", path "/admin".
func NewPanel() *Panel {
	return &Panel{
		id:              "admin",
		path:            "/admin",
		brand:           "Admin",
		themeMode:       ThemeSystem,
		maxContentWidth: "7xl",
		colors:          color.Defaults(),
		bySlug:          map[string]*resourceEntry{},
		authTable:       "users",
		emailColumn:     "email",
		passwordColumn:  "password",
		nameColumn:      "name",
		canAccess:       func(table.Row) bool { return true },
		passwordRules:   "min:8",
		resetTable:      "password_reset_tokens",
		resetExpiry:     60 * time.Minute,
	}
}

// ID identifies the panel (used to namespace its session and preferences).
func (p *Panel) ID(id string) *Panel { p.id = id; return p }

// Path sets where the panel is mounted ("/admin", or "" for the site root).
func (p *Panel) Path(path string) *Panel {
	p.path = strings.TrimRight("/"+strings.Trim(path, "/"), "/")
	return p
}

// BrandName is shown in the sidebar, page titles and auth pages.
func (p *Panel) BrandName(name string) *Panel { p.brand = name; return p }

// BaseURL is the absolute URL of the app (e.g. config app.url), used in e-mail links.
func (p *Panel) BaseURL(url string) *Panel { p.baseURL = strings.TrimRight(url, "/"); return p }

// DefaultThemeMode sets the initial theme; users can still toggle it.
func (p *Panel) DefaultThemeMode(mode ThemeMode) *Panel { p.themeMode = mode; return p }

// MaxContentWidth limits the page width: "full", "screen-2xl", "7xl", "6xl", "5xl"...
func (p *Panel) MaxContentWidth(width string) *Panel { p.maxContentWidth = width; return p }

// Colors overrides palettes by name: primary, gray, danger, success, warning, info.
func (p *Panel) Colors(colors map[string]color.Palette) *Panel {
	for name, palette := range colors {
		p.colors[name] = palette
	}
	return p
}

// NavigationGroups sets the order of sidebar groups.
func (p *Panel) NavigationGroups(groups ...string) *Panel {
	p.navigationGroups = append(p.navigationGroups, groups...)
	return p
}

// SidebarCollapsibleOnDesktop adds a button to collapse the sidebar to icons.
func (p *Panel) SidebarCollapsibleOnDesktop() *Panel { p.sidebarCollapse = true; return p }

// Resources registers CRUD resources.
func (p *Panel) Resources(resources ...Resource) *Panel {
	for _, r := range resources {
		p.resources = append(p.resources, newEntry(r))
	}
	return p
}

// Pages registers custom pages. A Dashboard is added automatically when no
// page uses the empty slug.
func (p *Panel) Pages(pages ...Page) *Panel { p.pages = append(p.pages, pages...); return p }

// Widgets registers the widgets shown on the Dashboard.
func (p *Panel) Widgets(widgets ...Widget) *Panel {
	p.widgets = append(p.widgets, widgets...)
	return p
}

// Plugin registers plugins.
func (p *Panel) Plugin(plugins ...Plugin) *Panel { p.plugins = append(p.plugins, plugins...); return p }

// Routes adds custom routes inside the panel, behind authentication.
// Paths are relative to the panel path.
func (p *Panel) Routes(fn func(r route.Router)) *Panel { p.routes = append(p.routes, fn); return p }

// Middleware runs on every panel route (after the session and CSRF middleware).
func (p *Panel) Middleware(m ...contractshttp.Middleware) *Panel {
	p.middleware = append(p.middleware, m...)
	return p
}

// AuthMiddleware runs on authenticated routes, after the login check.
func (p *Panel) AuthMiddleware(m ...contractshttp.Middleware) *Panel {
	p.authMiddleware = append(p.authMiddleware, m...)
	return p
}

// AuthTable configures which table and columns are used to log in.
func (p *Panel) AuthTable(tableName, emailColumn, passwordColumn, nameColumn string) *Panel {
	p.authTable, p.emailColumn, p.passwordColumn, p.nameColumn = tableName, emailColumn, passwordColumn, nameColumn
	return p
}

// CanAccess decides whether a user row may use the panel (like Filament's canAccessPanel).
func (p *Panel) CanAccess(fn func(user table.Row) bool) *Panel { p.canAccess = fn; return p }

// PasswordRules are the Goravel validation rules for new passwords (default "min:8").
func (p *Panel) PasswordRules(rules string) *Panel { p.passwordRules = rules; return p }

// Registration enables the "create account" page on the login screen.
// defaults are extra columns saved on every new user, e.g. {"role": "user"}.
// The new user is logged in only if CanAccess allows it.
func (p *Panel) Registration(defaults ...map[string]any) *Panel {
	p.registration = true
	p.newUserDefaults = map[string]any{}
	for _, d := range defaults {
		for k, v := range d {
			p.newUserDefaults[k] = v
		}
	}
	return p
}

// PasswordReset enables "forgot password" by e-mail (SMTP via Services.Mail).
// Requires the password reset tokens table (see the docs) and BaseURL.
func (p *Panel) PasswordReset() *Panel { p.passwordReset = true; return p }

// PasswordResetTable sets the tokens table (default "password_reset_tokens").
func (p *Panel) PasswordResetTable(name string) *Panel { p.resetTable = name; return p }

// PasswordResetExpiry sets how long a reset link is valid (default 60 minutes).
func (p *Panel) PasswordResetExpiry(d time.Duration) *Panel { p.resetExpiry = d; return p }

// URL builds a panel URL: p.URL("users", "create") => /admin/users/create.
func (p *Panel) URL(parts ...string) string {
	if len(parts) == 0 {
		return p.path + "/" // the dashboard route is registered with a trailing slash
	}
	return p.path + "/" + strings.Join(parts, "/")
}

// GetID returns the panel id.
func (p *Panel) GetID() string { return p.id }

// GetServices returns the services the panel was registered with.
func (p *Panel) GetServices() Services { return p.svc }

var (
	identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	slugRule   = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)*$`)
	widthRule  = regexp.MustCompile(`^(full|screen-(sm|md|lg|xl|2xl)|[2-7]?xl|lg|md|sm)$`)
)

// validate makes sure every developer-declared name is safe, since column
// names are interpolated into ORDER BY / search clauses.
func (p *Panel) validate() error {
	if !identifier.MatchString(p.id) {
		return fmt.Errorf("loom: invalid panel id %q", p.id)
	}
	if !widthRule.MatchString(p.maxContentWidth) {
		return fmt.Errorf("loom: invalid MaxContentWidth %q", p.maxContentWidth)
	}
	if p.passwordReset {
		if p.svc.Mail == nil {
			return fmt.Errorf("loom: PasswordReset() needs Services.Mail")
		}
		if p.baseURL == "" {
			return fmt.Errorf("loom: PasswordReset() needs BaseURL() for the e-mail link")
		}
	}

	names := []string{p.authTable, p.emailColumn, p.passwordColumn, p.nameColumn, p.resetTable}
	for k := range p.newUserDefaults {
		names = append(names, k)
	}

	used := map[string]string{}
	claim := func(slug, owner string) error {
		if prev, dup := used[slug]; dup {
			return fmt.Errorf("loom: slug %q used by %s and %s", slug, prev, owner)
		}
		used[slug] = owner
		return nil
	}
	for _, reserved := range []string{"login", "logout", "register", "forgot-password", "reset-password"} {
		used[reserved] = "the panel auth pages"
	}

	for _, r := range p.resources {
		if !slugRule.MatchString(r.Slug()) {
			return fmt.Errorf("loom: invalid resource slug %q", r.Slug())
		}
		if err := claim(r.Slug(), "resource "+r.Slug()); err != nil {
			return err
		}
		p.bySlug[r.Slug()] = r
		if !heroicons.Exists(r.IconName()) {
			return fmt.Errorf("loom: resource %q uses unknown heroicon %q (see https://heroicons.com)", r.Slug(), r.IconName())
		}
		names = append(names, r.TableName(), r.PrimaryKey())
		for _, c := range r.table.GetColumns() {
			names = append(names, c.Name())
		}
		names = append(names, r.table.GetSelect()...)
		for _, f := range r.table.GetFilters() {
			names = append(names, f.Name())
		}
		for _, f := range r.form.Fields(false) {
			names = append(names, f.Name())
			if u := f.UploadSettings(); u != nil && u.Storage == form.StorageDisk && p.svc.Storage == nil {
				return fmt.Errorf("loom: resource %q field %q stores files on disk and needs Services.Storage", r.Slug(), f.Name())
			}
		}
	}

	for _, pg := range p.pages {
		if pg.Slug() != "" && !slugRule.MatchString(pg.Slug()) {
			return fmt.Errorf("loom: invalid page slug %q", pg.Slug())
		}
		if err := claim(pg.Slug(), "page "+pg.Title()); err != nil {
			return err
		}
		if icon := pageIcon(pg); !heroicons.Exists(icon) {
			return fmt.Errorf("loom: page %q uses unknown heroicon %q", pg.Title(), icon)
		}
	}

	for _, n := range names {
		if !identifier.MatchString(n) {
			return fmt.Errorf("loom: invalid identifier %q", n)
		}
	}
	return nil
}

// Register runs the plugins, validates the configuration and mounts the routes.
func (p *Panel) Register(svc Services) error {
	p.svc = svc
	for _, plugin := range p.plugins {
		plugin.Register(p)
	}

	hasDashboard := false
	for _, pg := range p.pages {
		hasDashboard = hasDashboard || pg.Slug() == ""
	}
	if !hasDashboard {
		p.pages = append([]Page{&Dashboard{}}, p.pages...)
	}

	if err := p.validate(); err != nil {
		return err
	}
	render, err := newRenderer()
	if err != nil {
		return err
	}
	p.render = render

	var router route.Router = svc.Route
	if p.path != "" {
		router = router.Prefix(p.path)
	}
	base := router.Middleware(append([]contractshttp.Middleware{sessionmiddleware.StartSession(), &csrfMiddleware{}}, p.middleware...)...)

	base.Get("/login", p.showLogin)
	base.Post("/login", p.login)
	if p.registration {
		base.Get("/register", p.showRegister)
		base.Post("/register", p.register)
	}
	if p.passwordReset {
		base.Get("/forgot-password", p.showForgotPassword)
		base.Post("/forgot-password", p.sendResetLink)
		base.Get("/reset-password/{token}", p.showResetPassword)
		base.Post("/reset-password", p.resetPassword)
	}

	auth := base.Middleware(append([]contractshttp.Middleware{&authMiddleware{panel: p}}, p.authMiddleware...)...)
	auth.Group(func(r route.Router) {
		r.Post("/logout", p.logout)

		for _, pg := range p.pages {
			r.Get("/"+pg.Slug(), p.pageHandler(pg))
		}

		for _, res := range p.resources {
			h := &crudHandler{panel: p, res: res}
			slug := "/" + res.Slug()
			r.Get(slug, h.index)
			r.Get(slug+"/create", h.create)
			r.Post(slug, h.store)
			r.Post(slug+"/bulk-delete", h.bulkDelete)
			r.Get(slug+"/{id}/edit", h.edit)
			r.Post(slug+"/{id}", h.update)
			r.Post(slug+"/{id}/delete", h.destroy)
			r.Get(slug+"/{id}/files/{field}", h.file)
		}

		for _, fn := range p.routes {
			fn(r)
		}
	})

	return nil
}

// themeJSON is the Tailwind colors config injected in the layout.
func (p *Panel) themeJSON() string {
	b, _ := json.Marshal(p.colors)
	return string(b)
}
