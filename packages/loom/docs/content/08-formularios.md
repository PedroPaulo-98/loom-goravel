---
title: Formulários
section: Resources
lead: Os campos de criação e edição de um resource, com validação automática.
---

## O método Form

```go
import "goravel/packages/loom/form"

func (Resource) Form(f *form.Form) {
	f.Schema(
		form.Text("name").Label("Nome").Required().Rules("max:255"),
		form.Email("email").Label("E-mail").Required().Unique(),
		form.Password("password").Label("Senha").Required().Rules("min:8"),
		form.Select("role").Label("Perfil").Required().
			Option("admin", "Administrador").
			Option("user", "Usuário"),
		form.Toggle("is_active").Label("Ativo").Default("true"),
		form.Textarea("bio").Label("Sobre"),
	)
}
```

O nome passado para cada campo (`"name"`, `"email"`...) é o **nome da coluna** no banco.

## Layout

O formulário tem **duas colunas** em telas médias e grandes, e uma no celular. Para um campo ocupar a linha inteira:

```go
form.Text("address").ColumnSpanFull()
```

`Textarea` já ocupa a linha inteira por padrão.

## Campos

### Text

```go
form.Text("name").Label("Nome").Placeholder("Maria da Silva")
```

### Email

```go
form.Email("email").Required().Unique()
```

Valida o formato do e-mail automaticamente.

### Password

```go
form.Password("password").Required().Rules("min:8")
```

- O valor é salvo como **hash** (bcrypt, via `facades.Hash()`).
- A senha nunca é mostrada de volta no formulário.
- Na **edição**, o campo é opcional: deixar vazio mantém a senha atual.

### Number

```go
form.Number("year").Rules("min:1900", "max:2100")
```

Salvo como inteiro ou decimal. Vazio vira `NULL`.

### Textarea

```go
form.Textarea("notes").Placeholder("Observações...")
```

### Select

```go
form.Select("fuel").
	Option("flex", "Flex").
	Option("gasoline", "Gasolina")

// ou, a partir de um mapa (ordenado pelo rótulo):
form.Select("state").Options(map[string]string{"SP": "São Paulo", "RJ": "Rio de Janeiro"})
```

Só aceita valores das opções (regra `in:` automática). Vazio vira `NULL`.

### Toggle

```go
form.Toggle("is_active").Default("true")
```

Um switch liga/desliga, salvo como booleano.

### Date

```go
form.Date("birth_date")
```

Seletor de data nativo do navegador, salvo como `YYYY-MM-DD`.

### FileUpload

```go
form.FileUpload("avatar").Image()
```

Upload de arquivo em três modos: disco, base64 ou bytea. Tem uma página própria: [Upload de arquivos](upload-de-arquivos).

## Modificadores

| Método | Descrição |
|---|---|
| `.Label("Nome")` | Rótulo. Também é usado nas mensagens de erro ("O campo Nome..."). |
| `.Required()` | Obrigatório (mostra `*`). |
| `.Unique()` | O valor não pode se repetir na tabela. Na edição, ignora o próprio registro. |
| `.Rules("min:3", "max:255")` | Regras extras de validação do Goravel. |
| `.Default("valor")` | Valor inicial no formulário de criação. |
| `.Placeholder("...")` | Texto de exemplo dentro do campo. |
| `.Help("...")` | Texto de ajuda abaixo do campo. |
| `.Nullable()` | Campo vazio é salvo como `NULL` em vez de `""`. Necessário em colunas `uuid` e `json`. |
| `.OnlyOnCreate()` | Esconde o campo no formulário de edição. |
| `.ColumnSpanFull()` | Ocupa a linha inteira. |

## Validação

As regras são geradas a partir dos modificadores e passadas para o [validador do Goravel](https://www.goravel.dev/the-basics/validation.html):

| Declarado | Regra gerada |
|---|---|
| `.Required()` | `required` |
| `form.Email(...)` | `email` |
| `form.Number(...)` | `numeric` |
| `form.Date(...)` | `date` |
| `form.Select(...)` com opções | `in:a,b,c` |
| `.Unique()` | `unique:tabela,coluna` (`unique:tabela,coluna,id,{id}` na edição) |
| `.Rules(...)` | as regras como foram escritas |

Quando a validação falha, o formulário volta com:
- os valores digitados preservados (exceto senhas);
- a mensagem de erro embaixo de cada campo;
- a borda do campo em vermelho.

### Regras úteis do Goravel

```go
form.Text("cpf").Rules("len:11", "number")
form.Text("site").Rules("full_url")
form.Number("age").Rules("min:18", "max:120")
form.Text("code").Rules("regex:^[A-Z]{3}-[0-9]{4}$")
```

> As mensagens de erro padrão do Goravel são em inglês. Os nomes dos campos já usam o seu `Label`.
