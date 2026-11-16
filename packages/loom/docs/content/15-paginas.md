---
title: Páginas
section: Extensões
lead: Telas livres dentro do painel, com o menu, o tema e o login já prontos.
---

## Criando uma página

```go
package pages

import (
	"html/template"

	"goravel/app/loom/admin/widgets"
	"goravel/packages/loom"
)

type Reports struct{ loom.BasePage }

func (Reports) Slug() string            { return "reports" } // /admin/reports
func (Reports) Title() string           { return "Relatórios" }
func (Reports) Icon() string            { return "chart-bar" }
func (Reports) NavigationGroup() string { return "Relatórios" }

func (Reports) Content(ctx loom.Context) (template.HTML, error) {
	return ctx.Panel.RenderWidgets(ctx, []loom.Widget{
		widgets.UsersPerMonth(),
	})
}
```

Registre no provider:

```go
Pages(&pages.Reports{})
```

## Métodos

| Método | Obrigatório | Descrição |
|---|---|---|
| `Slug() string` | sim | URL da página. Pode ter subníveis (`relatorios/vendas`). `""` é a home do painel. |
| `Title() string` | sim | Título da página e nome no menu. |
| `Content(ctx) (template.HTML, error)` | sim | O HTML do conteúdo. |
| `Icon() string` | não | Ícone do menu (padrão `document-text`). |
| `NavigationGroup() string` | não | Grupo do menu. |
| `NavigationSort() int` | não | Ordem no grupo. |
| `HiddenFromNavigation() bool` | não | `true` esconde do menu. |

`loom.BasePage` fornece os métodos opcionais.

## Montando o conteúdo

### Com widgets

`ctx.Panel.RenderWidgets(ctx, widgets)` monta o mesmo grid do dashboard.

### Com html/template

```go
var tpl = template.Must(template.New("vendas").Parse(`
	<div class="rounded-xl border border-gray-200 bg-white p-6 dark:border-white/10 dark:bg-gray-900">
		<h2 class="font-semibold">Últimas vendas</h2>
		<ul class="mt-4 space-y-2">
			{{range .}}<li class="text-sm">{{.cliente}}: R$ {{.total}}</li>{{end}}
		</ul>
	</div>`))

func (Vendas) Content(ctx loom.Context) (template.HTML, error) {
	var rows []map[string]any
	// ... busque os dados com ctx.DB() ...
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, rows); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
```

O `html/template` escapa os dados automaticamente.

## Substituindo o Dashboard

O Dashboard padrão (`loom.Dashboard`) é uma página com o slug `""`. Para trocá-lo, registre uma página sua com o slug vazio:

```go
type Home struct{ loom.BasePage }

func (Home) Slug() string  { return "" }
func (Home) Title() string { return "Início" }
func (Home) Icon() string  { return "home" }
func (Home) Content(ctx loom.Context) (template.HTML, error) { /* ... */ }
```
