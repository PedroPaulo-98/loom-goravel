---
title: Upload de arquivos
section: Resources
lead: O campo FileUpload guarda arquivos de três jeitos. Escolha por campo onde o arquivo vai morar.
---

## Os três modos

| Modo | Onde fica o arquivo | O que a coluna guarda | Tipo da coluna |
|---|---|---|---|
| **Disco** (padrão) | arquivo físico em `storage/app/...` | o caminho (`avatars/3f9a...c1.png`) | `varchar` |
| **Base64** | dentro do banco | `data:image/png;base64,iVBORw0...` | `text` |
| **Binário** | dentro do banco | os bytes do arquivo | `bytea` (PostgreSQL) |

```go
form.FileUpload("avatar").Image().Directory("avatars")                             // disco
form.FileUpload("document").StoreAsBase64().AcceptedFileTypes("application/pdf")  // base64
form.FileUpload("contract").StoreAsBinary().AcceptedFileTypes("application/pdf")  // bytea
```

### Qual escolher?

- **Disco**: o melhor para arquivos grandes ou numerosos (fotos, anexos). O banco fica leve e o backup dos arquivos é separado.
- **Base64**: prático para arquivos pequenos que precisam viajar junto com o registro, como numa exportação JSON ou numa API que já espera base64. Ocupa cerca de 33% a mais que o arquivo original.
- **Binário (bytea)**: o arquivo fica no banco com o tamanho original, e o backup do banco já leva tudo. Bom para documentos pequenos e médios que precisam de consistência transacional com o registro.

## Migrations

```go
facades.Schema().Table("users", func(table schema.Blueprint) {
	table.String("avatar").Nullable()             // disco
	table.Text("document").Nullable()             // base64
	table.Column("contract", "bytea").Nullable()  // binário
})
```

> O Blueprint do Goravel não tem um método `Binary()`, então use `table.Column("coluna", "bytea")` no PostgreSQL. No MySQL, use `"longblob"`.

## Opções

| Método | Descrição |
|---|---|
| `.Disk("public")` | Disco do Goravel (`config/filesystems.go`). Padrão: `local` (`storage/app`). |
| `.Directory("avatars")` | Pasta dentro do disco. |
| `.StoreAsBase64()` | Guarda como data URI base64 na coluna. |
| `.StoreAsBinary()` | Guarda os bytes na coluna. |
| `.AcceptedFileTypes("image/*", "application/pdf")` | Tipos MIME aceitos. Aceita curingas como `image/*`. |
| `.MaxSize(2048)` | Tamanho máximo em KB. Padrão: 5120 (5 MB). |
| `.Image()` | Aceita só imagens (PNG, JPEG, GIF e WebP) e mostra o preview. |
| `.Required()` | Obrigatório. Na edição, basta já existir um arquivo. |
| `.Label()`, `.Help()` | Como nos outros campos. |

## Como funciona

### Ao salvar

1. Todos os campos são validados primeiro: texto, regras **e** arquivos (tamanho e tipo). Se algo falhar, nada é gravado.
2. O tipo do arquivo é detectado **pelo conteúdo**, não pelo nome nem pelo que o navegador informa. Um `.html` renomeado para `.pdf` é recusado.
3. No modo disco, o arquivo recebe um **nome aleatório** (`avatars/3f9a...c1.png`), para não haver colisão nem nomes maliciosos.
4. Se o banco falhar depois do arquivo ter sido gravado em disco, o arquivo é apagado (rollback).

### Na edição

- Sem um novo arquivo, o atual é mantido.
- Com um novo arquivo, o antigo é **substituído**; no modo disco, o arquivo antigo é apagado.
- Campos não obrigatórios mostram a opção **Remover**.
- Imagens mostram uma miniatura do arquivo atual.

### Ao excluir

Excluir um registro (individual ou em massa) também apaga os arquivos dele salvos em **disco**. Nos modos base64 e binário, o arquivo vai embora junto com a linha.

## Exibindo arquivos

Cada arquivo tem uma URL protegida por login:

```
GET /admin/{resource}/{id}/files/{campo}
```

Ela funciona igual para os três modos. Na tabela, use:

```go
table.Image("avatar")       // miniatura
table.FileLink("contract")  // link "Abrir"
```

### Segurança na entrega

- O tipo enviado ao navegador é detectado de novo a partir dos bytes. O valor salvo nunca é confiado.
- Só PNG, JPEG, GIF, WebP e PDF abrem no navegador. Qualquer outro tipo é baixado (`Content-Disposition: attachment`).
- A resposta vai com `X-Content-Type-Options: nosniff` e uma `Content-Security-Policy` com `sandbox`, para impedir que um arquivo execute scripts.

## Desempenho da listagem

Quando um resource tem campos base64 ou binários, a listagem **não** carrega o conteúdo dos arquivos: ela troca essas colunas por um indicador de "tem arquivo". Por isso, nesse caso, só as colunas declaradas na tabela são carregadas. Se um `FormatUsing` precisar de outra coluna, use `t.Select("outra_coluna")` (veja [Tabelas](tabelas#colunas-carregadas-na-listagem)).

## Exemplo completo

```go
func (Resource) Form(f *form.Form) {
	f.Schema(
		form.Text("name").Label("Nome").Required(),
		form.FileUpload("avatar").Label("Foto").
			Image().Directory("avatars").MaxSize(2048),
		form.FileUpload("document").Label("Documento (base64)").
			StoreAsBase64().AcceptedFileTypes("application/pdf", "image/*").MaxSize(1024),
		form.FileUpload("contract").Label("Contrato (bytea)").
			StoreAsBinary().AcceptedFileTypes("application/pdf").MaxSize(4096),
	)
}

func (Resource) Table(t *table.Table) {
	t.Columns(
		table.Image("avatar").Label("Foto"),
		table.Text("name").Label("Nome").Searchable(),
		table.FileLink("contract").Label("Contrato"),
	)
}
```

## Requisito

O modo disco precisa do serviço `Storage` no provider:

```go
Storage: facades.Storage(),
```
