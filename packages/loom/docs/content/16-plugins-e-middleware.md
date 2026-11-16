---
title: Plugins e middleware
section: Extensões
lead: Empacote funcionalidades para reaproveitar entre painéis, adicione rotas próprias e intercepte requisições.
---

## Plugins

Um plugin é um pacote que se registra no painel e pode adicionar resources, páginas, widgets e rotas. É o equivalente aos plugins do Filament:

```go
type CalendarPlugin struct{}

func (CalendarPlugin) ID() string { return "calendar" }

func (CalendarPlugin) Register(p *loom.Panel) {
	p.Pages(&CalendarPage{}).
		Widgets(&UpcomingEventsWidget{}).
		Routes(func(r route.Router) {
			r.Get("/calendar/events", eventsJSON)
		})
}
```

```go
Plugin(&CalendarPlugin{})
```

O `Register` roda antes da validação do painel, então tudo o que o plugin adiciona passa pelas mesmas checagens: slugs, ícones etc.

## Rotas próprias

Para endpoints extras (JSON para um gráfico, exportação CSV, ações customizadas):

```go
Routes(func(r route.Router) {
	r.Get("/reports/export", exportCSV)       // GET /admin/reports/export
	r.Post("/users/{id}/approve", approveUser) // POST /admin/users/5/approve
})
```

Essas rotas ficam **dentro** do painel:
- exigem login e passam pelo `CanAccess`;
- os POSTs exigem o token CSRF, enviado como campo `_token` ou header `X-CSRF-Token`.

Dentro de uma página do Loom, botões com HTMX já enviam o header sozinhos:

```html
<button hx-post="/admin/users/5/approve">Aprovar</button>
```

## Middleware

```go
Middleware(&middleware.Audit{})                 // todas as rotas do painel
AuthMiddleware(&middleware.RequireTwoFactor{})  // só as rotas logadas
```

| Método | Roda em | Ordem |
|---|---|---|
| `Middleware(...)` | todas as rotas, inclusive login, cadastro e reset | depois da sessão e do CSRF |
| `AuthMiddleware(...)` | só as rotas que exigem login | depois da checagem de login |

Um middleware do Goravel implementa `Handle` e `Signature`:

```go
type Audit struct{}

func (*Audit) Signature() string { return "audit" }

func (*Audit) Handle(ctx http.Context) {
	if ctx.Request().Method() == "POST" {
		facades.Log().Info("admin: " + ctx.Request().Path())
	}
	ctx.Request().Next()
}
```

## Dicas de organização

```
app/loom/admin/
├── user/           resources
├── cars/
├── pages/          páginas
├── widgets/        widgets
└── middleware/     middlewares do painel
```
