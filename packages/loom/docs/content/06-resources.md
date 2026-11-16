---
title: Resources
section: Resources
lead: Um resource é o CRUD completo de uma tabela. Você declara o formulário e a listagem, e o Loom gera as telas e as rotas.
---

## Estrutura

Cada resource fica numa pasta própria dentro do painel, e cada pasta é um pacote Go:

```
app/loom/admin/
├── user/
│   ├── resource.go
│   ├── form.go
│   └── table.go
├── address/
└── cars/
```

Os três arquivos declaram métodos do **mesmo tipo** `Resource`. Eles só estão separados por responsabilidade, como o Filament separa `UserResource`, `UserForm` e `UsersTable`.

## Criando um resource

O jeito mais rápido é o comando, que cria a pasta e registra o resource no painel. Veja [Gerando resources](gerando-resources):

```bash
go run . artisan loom:make-resource Car --generate
```

Para criar à mão:

```go
package cars

import "goravel/packages/loom"

type Resource struct {
	loom.BaseResource // fornece os métodos opcionais
}

func (Resource) Slug() string      { return "cars" }
func (Resource) TableName() string { return "cars" }
```

Depois registre no provider com `Resources(&cars.Resource{})`. O passo a passo completo está em [Primeiros passos](primeiros-passos).

## Métodos do Resource

| Método | Obrigatório | Padrão | Descrição |
|---|---|---|---|
| `Slug() string` | sim | — | Segmento da URL: `/admin/{slug}`. Só `a-z`, `0-9` e `-`. |
| `TableName() string` | sim | — | Tabela do banco. |
| `Form(f *form.Form)` | sim | — | Campos do formulário. Veja [Formulários](formularios). |
| `Table(t *table.Table)` | sim | — | Colunas e filtros. Veja [Tabelas](tabelas). |
| `Label() string` | não | slug formatado | Nome no plural: menu, títulos e card do dashboard. |
| `SingularLabel() string` | não | o `Label()` | Nome no singular: "Novo carro", "Carro excluído". |
| `Icon() string` | não | `rectangle-stack` | Ícone do menu. |
| `NavigationGroup() string` | não | `""` | Grupo do menu. |
| `NavigationSort() int` | não | `0` | Ordem dentro do grupo. |
| `PrimaryKey() string` | não | `id` | Chave primária. Pode ser inteira ou UUID. |
| `Timestamps() bool` | não | `true` | Preenche `created_at` e `updated_at`. |

### Tabelas sem timestamps

```go
func (Resource) Timestamps() bool { return false }
```

### Chave primária UUID

```go
func (Resource) PrimaryKey() string { return "uuid" }
```

O Loom converte IDs numéricos para inteiro e mantém os outros como texto.

## Rotas geradas

Para o slug `cars` no painel `/admin`:

| Método | URL | Ação |
|---|---|---|
| GET | `/admin/cars` | listagem |
| GET | `/admin/cars/create` | formulário de criação |
| POST | `/admin/cars` | salvar novo registro |
| GET | `/admin/cars/{id}/edit` | formulário de edição |
| POST | `/admin/cars/{id}` | salvar edição |
| POST | `/admin/cars/{id}/delete` | excluir |
| POST | `/admin/cars/bulk-delete` | excluir vários |
| GET | `/admin/cars/{id}/files/{campo}` | arquivo de um `FileUpload` |

Todas exigem login, e os POSTs exigem o token CSRF (os formulários do Loom já o enviam).

## Como os dados são gravados

O Loom grava direto na tabela, com o query builder do Goravel (`facades.DB()`). Não é preciso ter um model.

| Campo | Valor gravado |
|---|---|
| Texto, e-mail, textarea | string |
| Número | `int64` ou `float64`; `NULL` se vazio |
| Select, data | string; `NULL` se vazio |
| Toggle | `bool` |
| Senha | hash (`facades.Hash()`); vazio na edição mantém a senha atual |
| Arquivo | depende do modo; veja [Upload de arquivos](upload-de-arquivos) |

> Hooks de model (eventos, observers) não são disparados, porque o Loom não usa o ORM. Se precisar de regra de negócio ao salvar, use um middleware ou uma rota própria (veja [Plugins e middleware](plugins-e-middleware)).
