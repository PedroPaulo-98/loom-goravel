package loom

import (
	"html/template"
	"sort"

	contractsdb "github.com/goravel/framework/contracts/database/db"
	contractshttp "github.com/goravel/framework/contracts/http"
)

// Context is passed to pages and widgets while rendering.
type Context struct {
	HTTP  contractshttp.Context
	Panel *Panel
}

// DB returns the panel database bound to the request context.
func (c Context) DB() contractsdb.DB { return c.Panel.svc.DB.WithContext(c.HTTP) }

// Services returns every service the panel was registered with.
func (c Context) Services() Services { return c.Panel.svc }

// User returns the logged-in user row.
func (c Context) User() map[string]any { return currentUser(c.HTTP) }

// Page is a custom screen inside the panel (like a Filament Page).
//
// Embed BasePage and implement Slug, Title and Content:
//
//	type Reports struct{ loom.BasePage }
//	func (Reports) Slug() string  { return "reports" }
//	func (Reports) Title() string { return "Relatórios" }
//	func (Reports) Content(ctx loom.Context) (template.HTML, error) { ... }
type Page interface {
	// Slug is the URL segment ("" is the panel home, i.e. the dashboard).
	Slug() string
	Title() string
	Content(ctx Context) (template.HTML, error)
}

// Navigation is implemented by resources and pages that customise the sidebar.
// BaseResource and BasePage provide the defaults.
type Navigation interface {
	Icon() string
	NavigationGroup() string
	NavigationSort() int
}

// Hidden pages are routable but not listed in the sidebar.
type hiddenFromNavigation interface {
	HiddenFromNavigation() bool
}

// BasePage provides defaults for optional Page methods.
type BasePage struct{}

func (BasePage) Icon() string               { return "document-text" }
func (BasePage) NavigationGroup() string    { return "" }
func (BasePage) NavigationSort() int        { return 0 }
func (BasePage) HiddenFromNavigation() bool { return false }

func pageIcon(pg Page) string {
	if n, ok := pg.(Navigation); ok && n.Icon() != "" {
		return n.Icon()
	}
	return "document-text"
}

func (p *Panel) pageHandler(pg Page) contractshttp.HandlerFunc {
	return func(ctx contractshttp.Context) contractshttp.Response {
		content, err := pg.Content(Context{HTTP: ctx, Panel: p})
		if err != nil {
			return p.serverError(ctx, err)
		}
		return p.page(ctx, "page", 200, map[string]any{
			"Title":   pg.Title(),
			"Content": content,
		})
	}
}

// Dashboard is the default home page: it renders the panel widgets.
type Dashboard struct{ BasePage }

func (Dashboard) Slug() string        { return "" }
func (Dashboard) Title() string       { return "Dashboard" }
func (Dashboard) Icon() string        { return "home" }
func (Dashboard) NavigationSort() int { return -1000 }

func (Dashboard) Content(ctx Context) (template.HTML, error) {
	widgets := ctx.Panel.widgets
	if len(widgets) == 0 {
		widgets = []Widget{&ResourceCountsWidget{}}
	}
	return ctx.Panel.RenderWidgets(ctx, widgets)
}

// Widget is a block rendered on the dashboard (or on any page via RenderWidgets).
type Widget interface {
	Render(ctx Context) (template.HTML, error)
}

// Optional widget settings.
type (
	// WidgetSort orders widgets (lower first).
	WidgetSort interface{ Sort() int }
	// WidgetSpan sets the grid columns used: 1, 2 or 3 (3 = full width).
	WidgetSpan interface{ ColumnSpan() int }
)

// RenderWidgets renders widgets in a responsive 3-column grid.
func (p *Panel) RenderWidgets(ctx Context, widgets []Widget) (template.HTML, error) {
	sorted := append([]Widget(nil), widgets...)
	sort.SliceStable(sorted, func(i, j int) bool { return widgetSort(sorted[i]) < widgetSort(sorted[j]) })

	type cell struct {
		HTML template.HTML
		Span int
	}
	cells := make([]cell, 0, len(sorted))
	for _, w := range sorted {
		html, err := w.Render(ctx)
		if err != nil {
			return "", err
		}
		span := 3
		if s, ok := w.(WidgetSpan); ok && s.ColumnSpan() >= 1 && s.ColumnSpan() <= 3 {
			span = s.ColumnSpan()
		}
		cells = append(cells, cell{HTML: html, Span: span})
	}
	return p.render.fragment("widgets", map[string]any{"Cells": cells})
}

func widgetSort(w Widget) int {
	if s, ok := w.(WidgetSort); ok {
		return s.Sort()
	}
	return 0
}
