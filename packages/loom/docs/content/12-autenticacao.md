---
title: Autenticação
section: Usuários
lead: Login, controle de acesso ao painel, cadastro de novos usuários e o comando para criar administradores.
---

## Login

O login vem sempre ativo, em `/admin/login`. Ele usa:
- a **sessão** do Goravel (`SESSION_DRIVER`), não JWT;
- a tabela `users`, com as colunas `email`, `password` (hash) e `name`.

Para outra tabela ou outras colunas:

```go
AuthTable("funcionarios", "email_corporativo", "senha_hash", "nome_completo")
```

## Quem pode entrar: CanAccess

`CanAccess` recebe a linha do usuário e decide se ele entra no painel. É o equivalente ao `canAccessPanel()` do Filament:

```go
CanAccess(func(u table.Row) bool {
	return table.Stringify(u["role"]) == "admin" &&
		table.Boolean("is_active").Truthy(u)
})
```

A regra é conferida:
- **no login**: um usuário bloqueado vê "Credenciais inválidas ou sem acesso ao painel";
- **em toda requisição**: desativar um usuário corta o acesso dele na hora, mesmo se ele estiver logado.

Sem `CanAccess`, qualquer usuário da tabela pode entrar.

## Cadastro ("Criar conta")

```go
Registration(map[string]any{"role": "user", "is_active": true})
```

Ativa a tela `/admin/register` e o link **Criar conta** no login. O formulário pede nome, e-mail (único) e senha com confirmação. O mapa define colunas extras gravadas em todo usuário novo.

Depois do cadastro:
- se o novo usuário **passa** no `CanAccess`, ele já entra logado no painel;
- se **não passa**, volta para o login com a mensagem "Conta criada! Um administrador precisa liberar o seu acesso ao painel."

> Isso protege o painel: com `role = "user"` no cadastro e `CanAccess` exigindo `role = "admin"`, ninguém vira administrador só se cadastrando. Um admin libera o acesso editando o usuário.

### Regras de senha

```go
PasswordRules("min:10")
```

Vale para o cadastro e para a redefinição de senha. O padrão é `min:8`.

## Criar administradores pelo terminal

```bash
go run . artisan loom:make-user
```

Ou sem perguntas:

```bash
go run . artisan loom:make-user --name="Ana" --email=ana@example.com --password=uma-senha-forte
```

As colunas extras do usuário criado (como `role = admin`) são definidas no provider:

```go
loom.NewMakeUserCommand(r.Services, map[string]any{"role": "admin", "is_active": true})
```

## Logout

O botão de sair fica no topo, à direita. Ele invalida a sessão.

## Usuário logado no código

Em widgets e páginas:

```go
func (Reports) Content(ctx loom.Context) (template.HTML, error) {
	user := ctx.User() // map com as colunas do usuário
	nome := table.Stringify(user["name"])
	// ...
}
```

## Segurança

- O ID da sessão é regenerado no login (contra *session fixation*).
- A sessão é separada por painel (`ID`).
- Todo POST exige o token CSRF, inclusive login, cadastro e reset.
- Senhas são gravadas com bcrypt e nunca reexibidas.
