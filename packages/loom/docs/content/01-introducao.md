---
title: Introdução
section: Começando
lead: O Loom é um construtor de painéis administrativos para Goravel, inspirado no Filament do Laravel. Você declara o que quer ver, e o Loom monta as telas.
---

## O que é o Loom?

Com o Loom, você descreve as suas tabelas em Go (quais campos o formulário tem, quais colunas a listagem mostra) e o painel gera sozinho:

- listagem com **busca, filtros, ordenação e paginação**;
- formulários de **criação e edição** com validação;
- **exclusão** individual e em massa;
- **upload de arquivos** em disco, em base64 ou em binário (`bytea`);
- **dashboard** com cards de estatística e gráficos;
- **login, cadastro e recuperação de senha** por e-mail;
- **modo escuro**, cores personalizáveis e menu com grupos.

Tudo é renderizado no servidor com templates Go, Tailwind CSS e [HTMX](https://htmx.org). Não há build de JavaScript nem SPA para manter.

## Por que "Loom"?

O nome **Goravel** vem de *ravel*: emaranhar ou desembaraçar fios. O **Filament** é, literalmente, um fio fino. E o **Loom** é o *tear*, a máquina que tece os fios num tecido só.

No Loom, os fios são os seus resources, páginas e widgets, e o tecido é o painel.

## Conceitos principais

| Conceito | O que é | Equivalente no Filament |
|---|---|---|
| **Panel Provider** | Configura um painel: caminho, marca, cores, tema, login, menu. | `AdminPanelProvider` |
| **Resource** | Um CRUD completo de uma tabela do banco. | `Resource` |
| **Form** | Os campos de criação e edição de um resource. | `Form` / `Schema` |
| **Table** | As colunas, filtros e a ordenação da listagem. | `Table` |
| **Widget** | Um bloco do dashboard: estatísticas, gráficos ou HTML livre. | `Widget` |
| **Page** | Uma tela livre dentro do layout do painel. | `Page` |
| **Plugin** | Um pacote reutilizável de resources, páginas, widgets e rotas. | `Plugin` |

## Um gostinho

Um resource completo cabe em poucas linhas:

```go
func (Resource) Form(f *form.Form) {
	f.Schema(
		form.Text("name").Label("Nome").Required(),
		form.Email("email").Label("E-mail").Required().Unique(),
		form.FileUpload("avatar").Label("Foto").Image(),
	)
}

func (Resource) Table(t *table.Table) {
	t.Columns(
		table.Image("avatar"),
		table.Text("name").Label("Nome").Searchable().Sortable(),
		table.Text("email").Label("E-mail").Searchable(),
	)
}
```

## Onde fica cada coisa

```
packages/loom/                       o Loom (o "framework")
├── panel.go, crud.go, auth.go ...
├── form/  table/  color/  heroicons/
└── docs/                            esta documentação
app/
├── providers/admin_panel_provider.go   o seu painel
└── loom/admin/                         os seus resources, páginas e widgets
```

## Próximos passos

- [Instalação](instalacao): coloque o Loom num projeto Goravel.
- [Primeiros passos](primeiros-passos): crie o seu primeiro resource em cinco minutos.
