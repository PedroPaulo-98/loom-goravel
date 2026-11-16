// Package docs serves the Loom documentation site (like filamentphp.com/docs).
//
// Pages are Markdown files embedded from content/, named "NN-slug.md", with a
// small front matter block:
//
//	---
//	title: Instalação
//	section: Começando
//	---
//
// Mount it on any router:
//
//	docs.Register(facades.Route(), "/loom/docs")
package docs

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"sort"
	"strings"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"goravel/packages/loom/heroicons"
)

//go:embed content/*.md layout.html
var files embed.FS

// Version is shown next to the logo.
const Version = "0.1"

// sections defines the sidebar order; pages declare their section in the front matter.
var sections = []string{"Começando", "Painel", "Resources", "Personalização", "Usuários", "Extensões"}

type heading struct {
	Level int
	ID    string
	Text  string
}

type page struct {
	Slug     string
	Title    string
	Section  string
	Lead     string
	HTML     template.HTML
	Headings []heading
	text     string // plain text, for search
}

type navSection struct {
	Title string
	Pages []*page
}

// Site is a parsed documentation site.
type Site struct {
	base   string
	pages  []*page
	bySlug map[string]*page
	nav    []navSection
	search template.JS
	tmpl   *template.Template
}

// Register parses the embedded pages and mounts the site at base (e.g. "/loom/docs").
func Register(router route.Router, base string) (*Site, error) {
	s, err := New(base)
	if err != nil {
		return nil, err
	}
	router.Get(s.base, func(ctx contractshttp.Context) contractshttp.Response {
		return ctx.Response().Redirect(302, s.base+"/"+s.pages[0].Slug)
	})
	router.Get(s.base+"/{page}", s.handle)
	return s, nil
}

// New parses the embedded documentation.
func New(base string) (*Site, error) {
	s := &Site{base: "/" + strings.Trim(base, "/"), bySlug: map[string]*page{}}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)

	entries, err := fs.Glob(files, "content/*.md")
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	for _, name := range entries {
		raw, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		p, err := parsePage(md, path.Base(name), raw)
		if err != nil {
			return nil, fmt.Errorf("docs: %s: %w", name, err)
		}
		if _, dup := s.bySlug[p.Slug]; dup {
			return nil, fmt.Errorf("docs: duplicated slug %q", p.Slug)
		}
		s.bySlug[p.Slug] = p
		s.pages = append(s.pages, p)
	}
	if len(s.pages) == 0 {
		return nil, fmt.Errorf("docs: no pages")
	}

	// Sidebar sections in the declared order; pages keep file order inside a section.
	index := map[string]int{}
	for i, title := range sections {
		index[title] = i
		s.nav = append(s.nav, navSection{Title: title})
	}
	for _, p := range s.pages {
		i, ok := index[p.Section]
		if !ok {
			index[p.Section] = len(s.nav)
			i = len(s.nav)
			s.nav = append(s.nav, navSection{Title: p.Section})
		}
		s.nav[i].Pages = append(s.nav[i].Pages, p)
	}
	var nav []navSection
	var ordered []*page
	for _, sec := range s.nav {
		if len(sec.Pages) > 0 {
			nav = append(nav, sec)
			ordered = append(ordered, sec.Pages...)
		}
	}
	s.nav, s.pages = nav, ordered

	// Client-side search index: page titles and headings.
	type entry struct {
		Title   string `json:"t"`
		Section string `json:"s"`
		URL     string `json:"u"`
		Text    string `json:"x,omitempty"` // searched, not shown
	}
	var idx []entry
	for _, p := range s.pages {
		idx = append(idx, entry{p.Title, p.Section, s.base + "/" + p.Slug, p.Lead + " " + p.text})
		for _, h := range p.Headings {
			idx = append(idx, entry{h.Text, p.Title, s.base + "/" + p.Slug + "#" + h.ID, ""})
		}
	}
	b, _ := json.Marshal(idx)
	s.search = template.JS(b)

	s.tmpl, err = template.New("layout.html").Funcs(template.FuncMap{"icon": heroicons.SVG}).ParseFS(files, "layout.html")
	if err != nil {
		return nil, err
	}
	return s, nil
}

func parsePage(md goldmark.Markdown, filename string, raw []byte) (*page, error) {
	slug := strings.TrimSuffix(filename, ".md")
	if i := strings.Index(slug, "-"); i > 0 && strings.Trim(slug[:i], "0123456789.") == "" { // "06-x", "06.1-x"
		slug = slug[i+1:]
	}
	p := &page{Slug: slug, Section: "Outros"}

	body := raw
	if bytes.HasPrefix(raw, []byte("---\n")) {
		end := bytes.Index(raw[4:], []byte("\n---\n"))
		if end < 0 {
			return nil, fmt.Errorf("unterminated front matter")
		}
		for _, line := range strings.Split(string(raw[4:4+end]), "\n") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch strings.TrimSpace(k) {
			case "title":
				p.Title = v
			case "section":
				p.Section = v
			case "lead":
				p.Lead = v
			}
		}
		body = raw[4+end+5:]
	}
	if p.Title == "" {
		p.Title = slug
	}

	doc := md.Parser().Parse(text.NewReader(body), parser.WithContext(parser.NewContext(parser.WithIDs(newIDs()))))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering && (h.Level == 2 || h.Level == 3) {
			id, _ := h.AttributeString("id")
			idStr := ""
			if b, ok := id.([]byte); ok {
				idStr = string(b)
			}
			p.Headings = append(p.Headings, heading{Level: h.Level, ID: idStr, Text: nodeText(h, body)})
		}
		return ast.WalkContinue, nil
	})
	p.text = searchText(doc, body)
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, body, doc); err != nil {
		return nil, err
	}
	p.HTML = template.HTML(buf.String()) // goldmark escapes raw HTML (unsafe mode is off)
	return p, nil
}

func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering { // code spans also hold Text children
			b.Write(t.Segment.Value(src))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func (s *Site) handle(ctx contractshttp.Context) contractshttp.Response {
	p, ok := s.bySlug[ctx.Request().Route("page")]
	if !ok {
		return ctx.Response().Redirect(302, s.base+"/"+s.pages[0].Slug)
	}
	var prev, next *page
	for i, x := range s.pages {
		if x == p {
			if i > 0 {
				prev = s.pages[i-1]
			}
			if i < len(s.pages)-1 {
				next = s.pages[i+1]
			}
		}
	}
	var buf bytes.Buffer
	err := s.tmpl.Execute(&buf, map[string]any{
		"Base":    s.base,
		"Version": Version,
		"Page":    p,
		"Nav":     s.nav,
		"Prev":    prev,
		"Next":    next,
		"Search":  s.search,
	})
	if err != nil {
		return ctx.Response().String(500, "docs: %v", err)
	}
	return ctx.Response().Data(200, "text/html; charset=utf-8", buf.Bytes())
}

// ids generates readable heading anchors: "O método Table" => "o-metodo-table".
type ids struct{ used map[string]int }

func newIDs() *ids { return &ids{used: map[string]int{}} }

var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "è", "e", "ë", "e",
	"í", "i", "î", "i", "ì", "i", "ï", "i",
	"ó", "o", "ô", "o", "õ", "o", "ò", "o", "ö", "o",
	"ú", "u", "û", "u", "ù", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

func (r *ids) Generate(value []byte, _ ast.NodeKind) []byte {
	s := accents.Replace(strings.ToLower(string(value)))
	var b strings.Builder
	dash := false
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteRune(c)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	id := strings.TrimRight(b.String(), "-")
	if id == "" {
		id = "secao"
	}
	if n := r.used[id]; n > 0 {
		r.used[id] = n + 1
		id = fmt.Sprintf("%s-%d", id, n)
	} else {
		r.used[id] = 1
	}
	return []byte(id)
}

func (r *ids) Put(value []byte) { r.used[string(value)]++ }

// searchText collects the unique lower-case words of a page (prose and code).
func searchText(doc ast.Node, src []byte) string {
	seen := map[string]bool{}
	var words []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var chunk string
		switch t := n.(type) {
		case *ast.Text:
			chunk = string(t.Segment.Value(src))
		case *ast.FencedCodeBlock:
			var b strings.Builder
			for i := 0; i < t.Lines().Len(); i++ {
				seg := t.Lines().At(i)
				b.Write(seg.Value(src))
			}
			chunk = b.String()
		}
		for _, w := range strings.FieldsFunc(strings.ToLower(chunk), func(r rune) bool {
			return !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127)
		}) {
			if len(w) > 2 && !seen[w] {
				seen[w] = true
				words = append(words, w)
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(words, " ")
}
