---
title: Primeiros passos
section: Começando
lead: Um tutorial de ponta a ponta. Vamos criar um CRUD de carros, da migration até a tela pronta.
---

## O que vamos construir

Um resource **Carros** em `/admin/cars`, com:
- marca, modelo, ano e combustível;
- uma foto;
- busca, filtro por combustível e ordenação.

## 1. Criar a tabela

```bash
go run . artisan make:migration create_cars_table
```

```go
func (r *M20260101000000CreateCarsTable) Up() error {
	return facades.Schema().Create("cars", func(table schema.Blueprint) {
		table.ID()
		table.String("brand")
		table.String("model")
		table.Integer("year")
		table.String("fuel")
		table.String("photo").Nullable()
		table.Boolean("is_available").Default(true)
		table.TimestampsTz()
	})
}
```

```bash
go run . artisan migrate
```

## 2. Criar a pasta do resource

> Atalho: `go run . artisan loom:make-resource Car --generate` faz os passos 2 e 3 de uma vez, lendo as colunas da tabela. Veja [Gerando resources](gerando-resources). Abaixo está o caminho manual, para entender cada arquivo.

Cada resource tem uma pasta própria, com três arquivos:

```
app/loom/admin/cars/
├── resource.go   identidade: URL, tabela, nomes, ícone
├── form.go       formulário
└── table.go      listagem
```

### resource.go

```go
package cars

import "goravel/packages/loom"

type Resource struct {
	loom.BaseResource
}

func (Resource) Slug() string          { return "cars" }
func (Resource) TableName() string     { return "cars" }
func (Resource) Label() string         { return "Carros" }
func (Resource) SingularLabel() string { return "Carro" }
func (Resource) Icon() string          { return "truck" }
```

### form.go

```go
package cars

import "goravel/packages/loom/form"

func (Resource) Form(f *form.Form) {
	f.Schema(
		form.Text("brand").Label("Marca").Required(),
		form.Text("model").Label("Modelo").Required(),
		form.Number("year").Label("Ano").Required().Rules("min:1900"),
		form.Select("fuel").Label("Combustível").Required().
			Option("flex", "Flex").
			Option("gasoline", "Gasolina").
			Option("electric", "Elétrico"),
		form.FileUpload("photo").Label("Foto").Image().Directory("cars"),
		form.Toggle("is_available").Label("Disponível").Default("true"),
	)
}
```

### table.go

```go
package cars

import "goravel/packages/loom/table"

func (Resource) Table(t *table.Table) {
	t.Columns(
		table.Image("photo").Label("Foto"),
		table.Text("brand").Label("Marca").Sortable().Searchable(),
		table.Text("model").Label("Modelo").Sortable().Searchable(),
		table.Text("year").Label("Ano").Sortable(),
		table.Badge("fuel", map[string]string{"electric": "success", "flex": "info"}).Label("Combustível"),
		table.Boolean("is_available").Label("Disponível"),
	).Filters(
		table.SelectFilter("fuel").Label("Combustível").
			Option("flex", "Flex").
			Option("gasoline", "Gasolina").
			Option("electric", "Elétrico"),
	).DefaultSort("year", "desc")
}
```

## 3. Registrar no painel

No `AdminPanelProvider`:

```go
import "goravel/app/loom/admin/cars"

// ...
Resources(
	&user.Resource{},
	&cars.Resource{},
)
```

## 4. Pronto

Reinicie o app e abra `/admin/cars`. Você já tem:

| Tela | URL |
|---|---|
| Listagem | `/admin/cars` |
| Criar | `/admin/cars/create` |
| Editar | `/admin/cars/{id}/edit` |
| Arquivo da foto | `/admin/cars/{id}/files/photo` |

O dashboard também ganha um card com o total de carros, se você não configurou widgets próprios.

## Para onde ir agora

- [Tabelas](tabelas): todas as colunas e filtros.
- [Formulários](formularios): todos os campos e validações.
- [Upload de arquivos](upload-de-arquivos): disco, base64 ou bytea.
- [Cores e tema](cores-e-tema): deixe o painel com a sua cara.
