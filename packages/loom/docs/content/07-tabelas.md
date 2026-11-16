---
title: Tabelas
section: Resources
lead: As colunas, a busca, os filtros, a ordenação e a paginação da listagem de um resource.
---

## O método Table

```go
import "goravel/packages/loom/table"

func (Resource) Table(t *table.Table) {
	t.Columns(
		table.Image("avatar").Label("Foto"),
		table.Text("name").Label("Nome").Sortable().Searchable(),
		table.Badge("role", map[string]string{"admin": "primary"}).Label("Perfil"),
		table.Boolean("is_active").Label("Ativo"),
		table.DateTime("created_at").Label("Criado em").Sortable(),
	).Filters(
		table.SelectFilter("role").Label("Perfil").Option("admin", "Administrador").Option("user", "Usuário"),
	).DefaultSort("created_at", "desc").PerPage(25)
}
```

## Colunas

### Text

```go
table.Text("name").Label("Nome")
```

Mostra o valor como texto.

### Badge

```go
table.Badge("status", map[string]string{
	"aprovado":  "success",
	"pendente":  "warning",
	"recusado":  "danger",
	"rascunho":  "gray",
})
```

Mostra uma "pílula" colorida. O mapa liga cada valor a uma cor: `primary`, `success`, `danger`, `warning`, `info` ou `gray`. Os nomes `green`, `red`, `yellow`, `blue` e `indigo` também funcionam, como apelidos. Um valor fora do mapa fica cinza.

### Boolean

```go
table.Boolean("is_active")
```

Mostra ✓ verde ou ✗ vermelho. Entende `true`/`false`, `1`/`0` e `t`/`f`.

### DateTime

```go
table.DateTime("created_at").Format("02/01/2006")
```

Formata datas com o layout do Go (padrão `02/01/2006 15:04`), no fuso horário do servidor.

### Image

```go
table.Image("avatar")
```

Mostra uma miniatura redonda de um campo `FileUpload`. Funciona com os três modos de armazenamento (disco, base64 e bytea). Clicar na miniatura abre a imagem.

### FileLink

```go
table.FileLink("contract").Label("Contrato")
```

Mostra um link **Abrir** para o arquivo, ou "—" se não houver arquivo.

## Opções das colunas

| Método | Descrição |
|---|---|
| `.Label("...")` | Cabeçalho da coluna. Padrão: o nome da coluna formatado. |
| `.Sortable()` | Permite ordenar clicando no cabeçalho (↑ ↓). |
| `.Searchable()` | Inclui a coluna na busca. |
| `.Format(layout)` | Layout de data (só `DateTime`). |
| `.FormatUsing(func)` | Formata o valor exibido. |

### Formatando valores

`FormatUsing` recebe o valor e a linha inteira:

```go
table.Text("price").Label("Preço").FormatUsing(func(v any, row table.Row) string {
	return "R$ " + table.Stringify(v)
})

table.Text("city").Label("Cidade").FormatUsing(func(v any, row table.Row) string {
	return table.Stringify(v) + " / " + table.Stringify(row["state"])
})
```

`table.Stringify` converte qualquer valor do banco (texto, número, data, bytes) para string.

## Busca

A caixa de busca aparece quando pelo menos uma coluna é `.Searchable()`. A busca:
- procura em todas as colunas buscáveis ao mesmo tempo (OR);
- ignora maiúsculas e minúsculas;
- atualiza a tabela enquanto você digita, sem recarregar a página.

> Use `.Searchable()` só em colunas de texto. A busca usa `LOWER(coluna) LIKE ?`.

## Filtros

```go
t.Filters(
	table.SelectFilter("role").Label("Perfil").
		Option("admin", "Administrador").
		Option("user", "Usuário"),
	table.SelectFilter("is_active").Label("Status").
		Option("true", "Ativo").
		Option("false", "Inativo"),
)
```

Cada filtro vira um select acima da tabela e aplica `WHERE coluna = valor`. Os valores `"true"` e `"false"` são enviados ao banco como booleanos. Só valores declarados com `.Option` são aceitos.

## Ordenação e paginação

```go
t.DefaultSort("created_at", "desc") // ordem inicial
t.PerPage(25)                       // registros por página (padrão 10)
```

Sem `DefaultSort`, a listagem ordena pela chave primária, decrescente. O usuário pode trocar entre 10, 25, 50 e 100 itens por página no rodapé.

Busca, filtros, ordenação e página ficam na URL (`?q=ana&filter_role=admin&sort=name&dir=asc&page=2`), então você pode compartilhar um link com a listagem filtrada.

## Ações

Toda listagem já tem:
- **Novo** (botão no topo);
- **Editar** e **Excluir** em cada linha;
- **Seleção múltipla** com **Excluir selecionados**.

As exclusões pedem confirmação, e excluir um registro também apaga os arquivos dele salvos em disco.

## Colunas carregadas na listagem

Normalmente a listagem carrega todas as colunas da tabela (`SELECT *`). Quando o resource tem campos `FileUpload` em **base64** ou **bytea**, o Loom carrega só as colunas declaradas na tabela, para não trazer o conteúdo dos arquivos de cada linha.

Nesse caso, se um `FormatUsing` precisar de outra coluna, peça explicitamente:

```go
t.Select("state", "country")
```
