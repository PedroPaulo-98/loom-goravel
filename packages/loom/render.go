package loom

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log"
	"sort"
	"strings"

	contractshttp "github.com/goravel/framework/contracts/http"

	"goravel/packages/loom/heroicons"
	"goravel/packages/loom/table"
)

//go:embed views/*.html views/auth/*.html views/widgets/*.html views/emails/*.html
var viewsFS embed.FS

type renderer struct {
	pages     map[string]*template.Template
	fragments *template.Template
}

var funcs = template.FuncMap{
	"lower":   strings.ToLower,
	"display": func(c *table.Column, row table.Row) string { return c.Display(row) },
	"truthy":  func(c *table.Column, row table.Row) bool { return c.Truthy(row) },
	"color":   func(c *table.Column, row table.Row) string { return c.Color(row) },
	"field":   func(m map[string]string, k string) string { return m[k] },
	"add":     func(a, b int) int { return a + b },
	"icon":    heroicons.SVG,
	"hasfile": func(c *table.Column, row table.Row) bool { return hasFileValue(row[c.Name()]) },
	"js":      func(s string) template.JS { return template.JS(s) },
	"themeOptions": func() []struct{ Mode, Icon, Label string } {
		return []struct{ Mode, Icon, Label string }{
			{"light", "sun", "Claro"},
			{"dark", "moon", "Escuro"},
			{"system", "computer-desktop", "Sistema"},
		}
	},
	"initial": func(v any) string {
		s, _ := v.(string)
		if s == "" {
			return ""
		}
		return strings.ToUpper(string([]rune(s)[0]))
	},
}

// Layout pages use views/layout.html; auth pages use views/auth/layout.html.
var (
	layoutPages = []string{"page", "index", "form", "error"}
	authPages   = []string{"login", "register", "forgot-password", "reset-password"}
)

func newRenderer() (*renderer, error) {
	r := &renderer{pages: map[string]*template.Template{}}

	base, err := template.New("").Funcs(funcs).ParseFS(viewsFS, "views/head.html", "views/layout.html")
	if err != nil {
		return nil, err
	}
	for _, name := range layoutPages {
		t, err := template.Must(base.Clone()).ParseFS(viewsFS, "views/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("loom: parse view %s: %w", name, err)
		}
		r.pages[name] = t
	}

	authBase, err := template.New("").Funcs(funcs).ParseFS(viewsFS, "views/head.html", "views/auth/layout.html")
	if err != nil {
		return nil, err
	}
	for _, name := range authPages {
		t, err := template.Must(authBase.Clone()).ParseFS(viewsFS, "views/auth/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("loom: parse view auth/%s: %w", name, err)
		}
		r.pages[name] = t
	}

	r.fragments, err = template.New("").Funcs(funcs).ParseFS(viewsFS, "views/widgets/*.html", "views/emails/*.html")
	if err != nil {
		return nil, err
	}
	return r, nil
}

// fragment renders a named partial (widgets, e-mails).
func (r *renderer) fragment(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	if err := r.fragments.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

type navItem struct {
	Label, URL, Icon string
	Active           bool
	sort             int
}

type navGroup struct {
	Label string
	Items []navItem
}

// navigation builds the sidebar: ungrouped items first, then the groups in the
// order given to NavigationGroups (unknown groups go last, alphabetically).
func (p *Panel) navigation(path string) []navGroup {
	groups := map[string][]navItem{}
	add := func(group string, item navItem) { groups[group] = append(groups[group], item) }
	meta := func(v any) (string, int) {
		if n, ok := v.(Navigation); ok {
			return n.NavigationGroup(), n.NavigationSort()
		}
		return "", 0
	}

	for _, pg := range p.pages {
		if h, ok := pg.(hiddenFromNavigation); ok && h.HiddenFromNavigation() {
			continue
		}
		u := p.URL(pg.Slug())
		if pg.Slug() == "" {
			u = p.URL()
		}
		group, order := meta(pg)
		add(group, navItem{Label: pg.Title(), URL: u, Icon: pageIcon(pg), sort: order,
			Active: path == u || (pg.Slug() == "" && path == p.path) || (pg.Slug() != "" && strings.HasPrefix(path, u+"/"))})
	}
	for _, r := range p.resources {
		u := p.URL(r.Slug())
		group, order := meta(r.Resource)
		add(group, navItem{Label: r.PluralLabel(), URL: u, Icon: r.IconName(), sort: order,
			Active: path == u || strings.HasPrefix(path, u+"/")})
	}

	order := []string{""}
	seen := map[string]bool{"": true}
	for _, g := range p.navigationGroups {
		if !seen[g] {
			order, seen[g] = append(order, g), true
		}
	}
	var rest []string
	for g := range groups {
		if !seen[g] {
			rest = append(rest, g)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)

	var out []navGroup
	for _, g := range order {
		items := groups[g]
		if len(items) == 0 {
			continue
		}
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].sort != items[j].sort {
				return items[i].sort < items[j].sort
			}
			return items[i].Label < items[j].Label
		})
		out = append(out, navGroup{Label: g, Items: items})
	}
	return out
}

// page renders a full HTML page with the shared layout data.
func (p *Panel) page(ctx contractshttp.Context, name string, status int, data map[string]any) contractshttp.Response {
	req := ctx.Request()

	data["Brand"] = p.brand
	data["PanelID"] = p.id
	data["HomeURL"] = p.URL()
	data["Nav"] = p.navigation(req.Path())
	data["ThemeMode"] = string(p.themeMode)
	data["ColorsJSON"] = p.themeJSON()
	data["ColorsGray"] = p.colors["gray"]
	data["MaxWidth"] = "max-w-" + p.maxContentWidth
	data["Collapsible"] = p.sidebarCollapse
	data["LogoutURL"] = p.URL("logout")
	data["LoginURL"] = p.URL("login")
	data["RegisterURL"] = ""
	if p.registration {
		data["RegisterURL"] = p.URL("register")
	}
	data["ForgotURL"] = ""
	if p.passwordReset {
		data["ForgotURL"] = p.URL("forgot-password")
	}
	data["ResetURL"] = p.URL("reset-password")
	if req.HasSession() {
		sess := req.Session()
		data["CSRF"] = sess.Token()
		data["FlashSuccess"] = sess.Get("flash_success")
		if _, set := data["FlashError"]; !set {
			data["FlashError"] = sess.Get("flash_error")
		}
	}
	if user := currentUser(ctx); user != nil {
		data["UserName"] = table.Stringify(user[p.nameColumn])
		data["UserEmail"] = table.Stringify(user[p.emailColumn])
	}

	t, ok := p.render.pages[name]
	if !ok {
		return ctx.Response().String(500, "loom: unknown view %s", name)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "document", data); err != nil {
		log.Printf("loom: render %s: %v", name, err)
		return ctx.Response().String(500, "Erro ao renderizar a página.")
	}
	return ctx.Response().Data(status, "text/html; charset=utf-8", buf.Bytes())
}

func (p *Panel) serverError(ctx contractshttp.Context, err error) contractshttp.Response {
	log.Printf("loom: %s %s: %v", ctx.Request().Method(), ctx.Request().Path(), err)
	return p.page(ctx, "error", 500, map[string]any{
		"Title":   "Erro",
		"Code":    500,
		"Message": "Algo deu errado. Verifique os logs do servidor.",
	})
}

func (p *Panel) notFound(ctx contractshttp.Context) contractshttp.Response {
	return p.page(ctx, "error", 404, map[string]any{
		"Title":   "Não encontrado",
		"Code":    404,
		"Message": "O registro que você procura não existe.",
	})
}
