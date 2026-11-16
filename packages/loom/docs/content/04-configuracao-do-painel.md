---
title: Configuração do painel
section: Painel
lead: Tudo o que muda as informações e o comportamento do painel fica no Panel Provider, num único método encadeado.
---

## O Panel Provider

O painel é configurado no método `Panel` do provider. É o mesmo formato do `AdminPanelProvider.php` do Filament:

```go
func (r *AdminPanelProvider) Panel(panel *loom.Panel) *loom.Panel {
	return panel.
		ID("admin").
		Path("/admin").
		BrandName("SIAC - SUPER FÁCIL").
		BaseURL(facades.Config().GetString("app.url")).
		DefaultThemeMode(loom.ThemeDark).
		MaxContentWidth("full").
		Colors(map[string]color.Palette{"primary": color.Sky}).
		NavigationGroups("Gestão de Pessoas", "Relatórios", "Administração").
		Registration(map[string]any{"role": "user"}).
		PasswordReset().
		Resources(&user.Resource{}).
		Widgets(widgets.UsersOverview()).
		SidebarCollapsibleOnDesktop()
}
```

## Alterando as informações do painel

### Nome (marca)

```go
BrandName("SIAC - SUPER FÁCIL")
```

O nome aparece:
- no topo do menu lateral;
- no título da aba do navegador (`Dashboard · SIAC - SUPER FÁCIL`);
- nas telas de login, cadastro e recuperação de senha;
- no assunto e no corpo dos e-mails.

A primeira letra do nome vira o "logo" quadrado.

Para tirar o nome do `.env`:

```go
BrandName(facades.Config().GetString("app.name", "Admin"))
```

```env
APP_NAME="SIAC - SUPER FÁCIL"
```

### Endereço (path)

```go
Path("/admin")    // http://seu-site/admin
Path("/painel")   // http://seu-site/painel
Path("")          // http://seu-site/  (na raiz)
```

> Se usar `Path("")`, remova a rota `/` de `routes/web.go`, porque o dashboard passa a ocupar a raiz.

### Identificador

```go
ID("admin")
```

O ID separa a sessão de login e as preferências salvas no navegador (tema e menu recolhido). Dois painéis com IDs diferentes não compartilham login.

### URL absoluta

```go
BaseURL("https://sistema.exemplo.com")
```

Usada para montar os links enviados por e-mail (recuperação de senha). Normalmente vem de `APP_URL`.

### Largura do conteúdo

```go
MaxContentWidth("full")
```

Valores aceitos: `full`, `screen-2xl`, `screen-xl`, `7xl` (padrão), `6xl`, `5xl`, `4xl`, `3xl`, `2xl`, `xl`, `lg`, `md` e `sm`.

## Referência completa

| Método | Filament | Descrição |
|---|---|---|
| `ID(id)` | `->id()` | Identificador do painel. |
| `Path(path)` | `->path()` | Onde o painel é montado. |
| `BrandName(nome)` | `->brandName()` | Nome exibido. |
| `BaseURL(url)` | — | URL absoluta para links de e-mail. |
| `DefaultThemeMode(modo)` | `->defaultThemeMode()` | `loom.ThemeLight`, `loom.ThemeDark` ou `loom.ThemeSystem`. |
| `MaxContentWidth(largura)` | `->maxContentWidth()` | Largura máxima do conteúdo. |
| `Colors(map)` | `->colors()` | Paletas de cor. Veja [Cores e tema](cores-e-tema). |
| `NavigationGroups(...)` | `->navigationGroups()` | Ordem dos grupos do menu. Veja [Navegação](navegacao). |
| `SidebarCollapsibleOnDesktop()` | `->sidebarCollapsibleOnDesktop()` | Menu recolhível. |
| `Resources(...)` | `->resources()` | Resources de CRUD. |
| `Pages(...)` | `->pages()` | Páginas customizadas. |
| `Widgets(...)` | `->widgets()` | Widgets do dashboard. |
| `Plugin(...)` | `->plugin()` | Plugins. |
| `Middleware(...)` | `->middleware()` | Middlewares de todas as rotas do painel. |
| `AuthMiddleware(...)` | `->authMiddleware()` | Middlewares das rotas logadas. |
| `Routes(func)` | — | Rotas próprias dentro do painel. |
| `CanAccess(func)` | `canAccessPanel()` | Quem pode entrar. |
| `AuthTable(...)` | — | Tabela e colunas do login. |
| `Registration(defaults)` | `->registration()` | Tela "Criar conta". |
| `PasswordReset()` | `->passwordReset()` | "Esqueceu a senha?" por e-mail. |
| `PasswordRules(regras)` | — | Regras para senhas novas (padrão `min:8`). |
| `PasswordResetExpiry(d)` | — | Validade do link de reset (padrão 60 min). |
| `PasswordResetTable(nome)` | — | Tabela de tokens (padrão `password_reset_tokens`). |

## Serviços

O painel recebe os serviços do Goravel por `loom.Services`. Cada um só é obrigatório se a funcionalidade correspondente estiver em uso:

| Campo | Facade | Obrigatório quando |
|---|---|---|
| `Route` | `facades.Route()` | sempre |
| `DB` | `facades.DB()` | sempre |
| `Validation` | `facades.Validation()` | sempre |
| `Hash` | `facades.Hash()` | sempre |
| `Mail` | `facades.Mail()` | com `PasswordReset()` |
| `Storage` | `facades.Storage()` | com `FileUpload` salvo em disco |

## Validação na inicialização

O Loom confere a configuração quando o app sobe e para com um erro claro, em vez de falhar no meio de uma requisição:
- slugs repetidos ou inválidos;
- ícones que não existem;
- nomes de tabela e coluna com caracteres inválidos;
- `PasswordReset()` sem `Mail` ou sem `BaseURL`;
- upload em disco sem `Storage`.

## Mais de um painel

Crie outro provider, por exemplo `CoordenadorPanelProvider`, com `ID("coordenador")` e `Path("/coordenador")`, e registre os dois em `bootstrap/providers.go`. Cada painel tem os seus resources, o seu `CanAccess` e o seu login.
