---
title: Instalação
section: Começando
lead: Requisitos, como registrar o painel no Goravel e como criar o primeiro usuário administrador.
---

## Requisitos

- Go 1.25 ou superior
- Goravel 1.18 (driver HTTP Gin)
- PostgreSQL ou MySQL, com as facades `Route`, `DB`, `Validation`, `Hash`, `Session` e `Storage` instaladas. `Mail` só é necessária para a recuperação de senha.

## 1. Copiar o pacote

Hoje o Loom vive dentro do projeto, em `packages/loom`. Para usar em outro projeto Goravel, copie a pasta e ajuste o caminho dos imports (`goravel/packages/loom`) para o módulo do novo projeto.

## 2. Criar o Panel Provider

Crie `app/providers/admin_panel_provider.go`:

```go
package providers

import (
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/foundation"

	"goravel/app/facades"
	"goravel/packages/loom"
)

type AdminPanelProvider struct{}

func (r *AdminPanelProvider) Panel(panel *loom.Panel) *loom.Panel {
	return panel.
		ID("admin").
		Path("/admin").
		BrandName(facades.Config().GetString("app.name"))
}

func (r *AdminPanelProvider) Services() loom.Services {
	return loom.Services{
		Route:      facades.Route(),
		DB:         facades.DB(),
		Validation: facades.Validation(),
		Hash:       facades.Hash(),
		Mail:       facades.Mail(),
		Storage:    facades.Storage(),
	}
}

func (r *AdminPanelProvider) Register(app foundation.Application) {}

func (r *AdminPanelProvider) Boot(app foundation.Application) {
	app.Commands([]console.Command{
		loom.NewMakeUserCommand(r.Services, map[string]any{"role": "admin"}),
	})
	if err := r.Panel(loom.NewPanel()).Register(r.Services()); err != nil {
		panic(err)
	}
}
```

## 3. Registrar o provider

Em `bootstrap/providers.go`, adicione o provider **no final da lista**, depois dos providers de rota, banco, e-mail e hash:

```go
func Providers() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{
		// ... providers do Goravel ...
		&providers.AdminPanelProvider{},
	}
}
```

## 4. Tabela de usuários

O login usa a tabela `users`, com as colunas `name`, `email` e `password`. Para usar outra tabela ou outras colunas, veja [Autenticação](autenticacao).

```go
facades.Schema().Create("users", func(table schema.Blueprint) {
	table.ID()
	table.String("name")
	table.String("email")
	table.String("password")
	table.String("role").Default("user")
	table.Boolean("is_active").Default(true)
	table.TimestampsTz()
	table.Unique("email")
})
```

```bash
go run . artisan migrate
```

## 5. Criar o primeiro administrador

```bash
go run . artisan loom:make-user
```

O comando pergunta nome, e-mail e senha. Para rodar sem perguntas (em CI, por exemplo):

```bash
go run . artisan loom:make-user --name="Ana" --email=ana@example.com --password=uma-senha-forte
```

## 6. Abrir o painel

```bash
go run .
```

Acesse `http://localhost:3000/admin` e entre com o usuário criado.

> Se o app parar ao subir com uma mensagem começando com `loom:`, é a validação de configuração do Loom. Ela aponta exatamente o que corrigir: um slug repetido, um ícone que não existe, um serviço faltando etc.

## Variáveis de ambiente úteis

```env
APP_NAME="Meu Sistema"          # nome mostrado no painel (BrandName)
APP_URL=http://localhost:3000   # usado nos links dos e-mails
SESSION_DRIVER=file             # o login usa a sessão do Goravel
```
