---
title: Autenticação da API
section: API
lead: Tokens Bearer com permissões, validade e revogação, no estilo do Laravel Sanctum.
---

## Como funciona

O cliente envia o token em todas as requisições:

```
Authorization: Bearer lm_2e4RZNvEQmgiZA6d39r9kosDXmgkndiLPtwK6hlZH7M
```

- **Armazenamento**: o banco guarda só o hash SHA-256 do token. Se o banco vazar, os tokens não podem ser usados.
- **Permissões**: cada token tem uma lista de permissões (*abilities*), como `cars:read` e `cars:write`.
- **Validade**: o token pode ter data de validade e pode ser revogado a qualquer momento.
- **Dono**: o token pertence a um usuário, e o `CanAccess` da API é conferido a cada requisição. Desativar o usuário desativa os tokens dele.

## Migration

A tabela `api_tokens` vem na migration `create_api_tokens_table`:

```bash
go run . artisan migrate
```

## Três formas de criar um token

### 1. Pelo painel

Registre a tela de tokens no `AdminPanelProvider`:

```go
Resources(
	&user.Resource{},
	&loom.APITokenResource{Group: "Sistema"},
)
```

Em **Tokens de API → Novo token de API**:
1. Informe o nome, as permissões e, se quiser, a validade.
2. O token aparece **uma única vez** na mensagem de sucesso. Copie na hora.

A lista mostra o último uso de cada token, e excluir um token revoga o acesso na mesma hora.

### 2. Pelo terminal

```bash
go run . artisan loom:make-token --email=admin@example.com --name=erp --abilities="cars:read,cars:write" --days=90
```

Sem `--days`, o token não expira.

### 3. Pela própria API (login)

Ligue com `TokenLogin()` no `APIProvider`:

```bash
curl -X POST http://localhost:3000/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"email": "ana@example.com", "password": "secret123", "name": "app-mobile"}'
```

```json
{
  "token": "lm_...",
  "token_type": "Bearer",
  "abilities": [],
  "expires_at": "2026-12-16T14:00:00Z"
}
```

Os tokens emitidos pelo login recebem as permissões passadas em `TokenLogin`:

```go
TokenLogin()                        // nenhuma: só rotas Authenticated() ou Public()
TokenLogin("cars:read")             // pode ler carros
TokenLogin("cars:read", "orders:*") // ler carros e tudo de pedidos
```

O padrão é nenhuma permissão. Assim, qualquer usuário que faça login não ganha acesso de escrita sem você decidir. Permissões maiores são dadas pelo painel ou pelo terminal.

## Rotas de autenticação

| Rota | O que faz |
|---|---|
| `POST /auth/token` | Troca e-mail e senha por um token (público, com limite de requisições). |
| `GET /auth/me` | Dono do token e dados do token. |
| `DELETE /auth/token` | Revoga o token em uso (logout). |

```json
GET /api/v1/auth/me

{
  "data": { "id": 1, "name": "Admin", "email": "admin@example.com" },
  "token": { "name": "app-mobile", "abilities": ["cars:read"], "expires_at": null }
}
```

## Permissões

| Permissão do token | Libera |
|---|---|
| `cars:read` | Só `api.Can("cars:read")` |
| `cars:*` | Qualquer `cars:...` |
| `*` | Tudo |

O nome das permissões é livre. O gerador usa `<slug>:read`, `<slug>:write` e `<slug>:delete`, mas você pode criar as suas, como `relatorios:exportar`.

## Usando o token no código

```go
func (Resource) BeforeDelete(ctx loom.Context, ids []any) error {
	if !ctx.Token().Can("cars:force-delete") {
		return loom.Halt("Excluir exige a permissão cars:force-delete.")
	}
	return nil
}
```

Em rotas próprias:

```go
Routes(func(r route.Router) {
	r.Get("/reports/sales", a.Guard(api.Can("reports:read"), salesReport))
})
```

## Boas práticas

- **Escopo mínimo**: dê a cada integração só as permissões de que ela precisa.
- **Validade**: use `--days` ou a data de validade em integrações temporárias.
- **Revogação**: revogue pelo painel tokens que vazaram ou não são mais usados. A coluna "Último uso" ajuda a encontrá-los.
- **HTTPS**: sirva a API com HTTPS em produção, porque o token vai em todas as requisições.
