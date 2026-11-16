// Package heroicons embeds Heroicons v2.2.0 (24px, outline and solid) and
// renders them by name. Heroicons is MIT licensed, see LICENSE.
//
// Accepted names (same convention as Filament):
//
//	"users"              outline (default)
//	"heroicon-o-users"   outline
//	"heroicon-s-users"   solid
//	"o-users", "s-users" short forms
//
// Browse the names at https://heroicons.com.
package heroicons

import (
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed outline/*.svg solid/*.svg
var files embed.FS

// resolve converts an icon name into its embedded file path.
func resolve(name string) (string, error) {
	n := strings.TrimSpace(strings.ToLower(name))
	n = strings.TrimPrefix(n, "heroicon-")

	style := "outline"
	switch {
	case strings.HasPrefix(n, "o-"):
		n = n[2:]
	case strings.HasPrefix(n, "s-"):
		style, n = "solid", n[2:]
	}
	if n == "" || strings.ContainsAny(n, "/\\.") {
		return "", fmt.Errorf("heroicons: invalid icon name %q", name)
	}
	return style + "/" + n + ".svg", nil
}

// Exists reports whether the icon name is a known Heroicon.
func Exists(name string) bool {
	path, err := resolve(name)
	if err != nil {
		return false
	}
	_, err = files.Open(path)
	return err == nil
}

// SVG returns the inline <svg> for name with the given CSS classes.
// Unknown names return an empty string.
func SVG(name, class string) template.HTML {
	path, err := resolve(name)
	if err != nil {
		return ""
	}
	raw, err := files.ReadFile(path)
	if err != nil {
		return ""
	}
	svg := strings.TrimSpace(string(raw))
	if class != "" {
		svg = strings.Replace(svg, "<svg ", `<svg class="`+template.HTMLEscapeString(class)+`" `, 1)
	}
	// The files are the unmodified Heroicons release, embedded at build time.
	return template.HTML(svg)
}
