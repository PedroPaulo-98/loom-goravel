---
title: Resources de API
section: API
lead: Exponha uma tabela como API REST em JSON, com busca, filtros, paginação, validação e regras de acesso por rota.
---

## Como funciona

Um resource de API é parecido com um resource do painel: você declara os campos e o Loom gera as rotas. A diferença é que ele responde JSON e é protegido por **tokens**, não por login com sessão.

```
app/providers/api_provider.go   configura a API (/api/v1)
app/loom/api/car/
├── resource.go   slug, tabela e regras de acesso
├── fields.go     campos aceitos e devolvidos
└── hooks.go      ganchos (opcional)
```

## Gerando pelo terminal

```bash
go run . artisan loom:make-resource Car --api --generate
```

O `--api` cria a pasta em `app/loom/api/` e registra o resource no `APIProvider`. Com `--generate`, os campos saem das colunas da tabela. As demais opções (`--table`, `--slug`, `--force`, `--no-register`) funcionam igual às do painel.

## O resource

```go
package car

import (
	"goravel/packages/loom"
	"goravel/packages/loom/api"
)

type Resource struct {
	loom.BaseAPIResource
}

func (Resource) Slug() string      { return "cars" }
func (Resource) TableName() string { return "cars" }

func (Resource) Access() api.Access {
	return api.Access{
		Index:   api.Public(),
		Show:    api.Public(),
		Store:   api.Can("cars:write"),
		Update:  api.Can("cars:write"),
		Destroy: api.Can("cars:delete"),
	}
}

func (Resource) Fields(s *api.Schema) {
	s.Schema(
		api.Field("brand").Required().Rules("max:100").Searchable().Sortable(),
		api.Field("model").Required().Searchable(),
		api.Field("year").Integer().Required().Rules("min:1950").Filterable().Sortable(),
		api.Field("price").Number().Sortable(),
		api.Field("available").Boolean().Filterable(),
		api.Field("cost_price").Number().Hidden(),
		api.Field("created_at").DateTime().ReadOnly().Sortable(),
	)
}
```

O `loom.BaseAPIResource` fornece a chave primária `id`, os timestamps automáticos e o acesso padrão.

## Rotas geradas

| Método e rota | Ação | Regra em `Access` |
|---|---|---|
| `GET /api/v1/cars` | Lista | `Index` |
| `GET /api/v1/cars/{id}` | Um registro | `Show` |
| `POST /api/v1/cars` | Cria | `Store` |
| `PUT /api/v1/cars/{id}` e `PATCH` | Atualiza | `Update` |
| `DELETE /api/v1/cars/{id}` | Exclui | `Destroy` |

## Público ou privado

Cada rota tem sua regra:

| Regra | Quem acessa |
|---|---|
| `api.Public()` | Qualquer um, sem token |
| `api.Authenticated()` | Qualquer token válido |
| `api.Can("cars:write")` | Tokens com essa permissão (ou `cars:*`, ou `*`) |

Rotas que você **não declarar** exigem token. Assim nada fica público por esquecimento. Para uma API só de leitura pública, use o atalho:

```go
func (Resource) Access() api.Access { return api.ReadOnly() } // Index e Show públicos
```

Numa rota pública, um token enviado é aceito se for válido, e `ctx.User()` fica disponível nos ganchos. Um token inválido numa rota pública é ignorado.

## Campos

Tipos (o padrão é texto):

| Método | JSON aceito | Exemplo |
|---|---|---|
| (nenhum) | texto | `"Gol"` |
| `.Integer()` | número inteiro | `2020` |
| `.Number()` | número decimal | `49990.90` |
| `.Boolean()` | `true`/`false` | `true` |
| `.Date()` | `"AAAA-MM-DD"` | `"2026-01-31"` |
| `.DateTime()` | RFC 3339 | `"2026-01-31T14:00:00Z"` |
| `.Password()` | texto, salvo com hash e nunca devolvido | |

Modificadores:

| Método | Efeito |
|---|---|
| `.Required()` | Obrigatório ao criar; não pode ser esvaziado ao atualizar. |
| `.Nullable()` | Aceita `null` e salva `NULL`. |
| `.Unique()` | O valor não pode se repetir (ignora o próprio registro ao atualizar). |
| `.Rules("min:3", "max:100")` | Regras de validação do Goravel. |
| `.Hidden()` | Pode ser gravado, mas nunca aparece na resposta. |
| `.ReadOnly()` | Aparece na resposta, mas não pode ser gravado (ids, timestamps). |
| `.Searchable()` | Entra no `?search=`. |
| `.Sortable()` | Permite `?sort=campo` e `?sort=-campo`. |
| `.Filterable()` | Permite `?filter[campo]=valor`. |

Só os campos declarados são lidos do banco e devolvidos. Colunas fora da lista, como senhas, nunca vazam.

## Listagem

```
GET /api/v1/cars?search=gol&filter[available]=true&filter[year]=2020,2021&sort=-price&page=2&per_page=20
```

| Parâmetro | Descrição |
|---|---|
| `search` | Busca em todos os campos `Searchable` (sem diferenciar maiúsculas). |
| `filter[campo]` | Igualdade. Vários valores separados por vírgula valem como "qualquer um". |
| `sort` | Campo `Sortable` ou a chave primária. Com `-` na frente, ordem decrescente. O padrão é `-id`. |
| `page` / `per_page` | Paginação. `per_page` vai de 1 a 100 (padrão 15). |

Resposta:

```json
{
  "data": [
    { "id": 7, "brand": "VW", "model": "Gol", "year": 2020, "price": 49990.9, "available": true, "created_at": "2026-11-16T14:00:00Z" }
  ],
  "meta": { "page": 2, "per_page": 20, "total": 87, "last_page": 5 }
}
```

As datas e horas saem sempre em UTC.

## Criar, atualizar e excluir

```bash
curl -X POST http://localhost:3000/api/v1/cars \
  -H "Authorization: Bearer SEU_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"brand": "VW", "model": "Gol", "year": 2020, "available": true}'
```

- **Criar** responde `201` com o registro criado.
- **Atualizar** responde `200` com o registro atualizado. `PUT` e `PATCH` são parciais: só os campos enviados mudam.
- **Excluir** responde `204`, sem corpo.
- **Corpo**: em JSON (`Content-Type: application/json`) ou formulário (`application/x-www-form-urlencoded`).

## Erros

Todas as respostas de erro têm `message`:

| Status | Quando |
|---|---|
| `400` | JSON inválido, filtro, ordenação ou `per_page` não permitidos |
| `401` | Sem token, ou token inválido, expirado ou revogado |
| `403` | O token não tem a permissão exigida |
| `404` | Registro não encontrado (ou fora do escopo de `ModifyQuery`) |
| `422` | Validação falhou, ou um gancho retornou `loom.Halt` |
| `429` | Limite de requisições atingido (veja o header `Retry-After`) |

Erros de validação trazem também `errors`, com as mensagens de cada campo:

```json
{
  "message": "Os dados enviados são inválidos.",
  "errors": { "year": ["O campo year deve ser um número inteiro."] }
}
```

## Ganchos

Os mesmos ganchos do painel funcionam na API, e geram a resposta `422` quando retornam `loom.Halt`. O `hooks.go` gerado traz todos comentados. Há um a mais, o `ModifyQuery`, que restringe **todas** as rotas (listar, ver, atualizar e excluir):

```go
// Cada token só enxerga e altera os carros do próprio dono.
func (Resource) ModifyQuery(ctx loom.Context, q contractsdb.Query) contractsdb.Query {
	return q.Where("user_id", ctx.User()["id"])
}

func (Resource) BeforeCreate(ctx loom.Context, data map[string]any) error {
	data["user_id"] = ctx.User()["id"]
	return nil
}
```

O `ModifyListQuery` continua existindo e afeta só a listagem.

Nos ganchos da API:
- `ctx.API` está preenchido, e `ctx.Panel` fica `nil`;
- `ctx.User()` é o dono do token, ou `nil` numa rota pública sem token;
- `ctx.Token()` traz o nome e as permissões do token em uso.

## Configurando a API

Em [`app/providers/api_provider.go`](https://github.com/PedroPaulo-98/loom-goravel/blob/main/app/providers/api_provider.go):

```go
func (r *APIProvider) API(a *loom.API) *loom.API {
	return a.
		Prefix("/api/v1").
		RateLimit(60).
		TokenLogin().
		TokenExpiry(30 * 24 * time.Hour).
		CanAccess(func(u table.Row) bool {
			return table.Boolean("is_active").Truthy(u)
		}).
		Resources(&car.Resource{})
}
```

| Método | Para que serve |
|---|---|
| `Prefix("/api/v1")` | Prefixo das rotas (padrão `/api`). |
| `RateLimit(60)` | Requisições por minuto por cliente: por token, ou por IP sem token. `0` desliga. |
| `TokenLogin(abilities...)` | Liga `POST /auth/token`, `DELETE /auth/token` e `GET /auth/me`. Veja [Autenticação da API](autenticacao-da-api). |
| `TokenExpiry(d)` | Validade dos tokens emitidos pelo login. |
| `CanAccess(fn)` | Quais usuários podem usar a API. Vale para o login e para todo token. |
| `AuthTable(tabela, email, senha)` | Tabela de usuários (padrão `users`, `email`, `password`). |
| `TokenTable("api_tokens")` | Tabela dos tokens. |
| `Middleware(...)` | Middlewares do Goravel em todas as rotas da API. |
| `Routes(fn)` | Rotas próprias. Proteja-as com `a.Guard(api.Can("..."), handler)`. |

As respostas têm os headers `X-RateLimit-Limit` e `X-RateLimit-Remaining`. Se a API for chamada direto do navegador, de outro domínio, ajuste o CORS em `config/cors.go`.
