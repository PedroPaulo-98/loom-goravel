# Loom: painel admin para Goravel

A documentação completa é um site servido pelo próprio app:

**http://localhost:3000/loom/docs**

As páginas são arquivos Markdown em [`packages/loom/docs/content`](../../packages/loom/docs/content). Para editar uma página, edite o `.md` e reinicie o app. Para criar uma página nova, adicione `NN-slug.md` com o front matter `title`, `section` e `lead`.

## Atalhos

| Assunto | Página |
|---|---|
| Instalação | `/loom/docs/instalacao` |
| Primeiro resource, passo a passo | `/loom/docs/primeiros-passos` |
| Gerar resource pelo terminal | `/loom/docs/gerando-resources` |
| Alterar nome, caminho, tema | `/loom/docs/configuracao-do-painel` |
| Tabelas | `/loom/docs/tabelas` |
| Formulários | `/loom/docs/formularios` |
| Upload (disco, base64, bytea) | `/loom/docs/upload-de-arquivos` |
| Cores (e onde adicionar mais) | `/loom/docs/cores-e-tema` |
| Login, cadastro, CanAccess | `/loom/docs/autenticacao` |
| Recuperação de senha (SMTP) | `/loom/docs/recuperacao-de-senha` |
| Widgets, páginas, plugins | `/loom/docs/widgets` |

## Neste projeto

```
app/providers/admin_panel_provider.go   configuração do painel /admin
app/loom/admin/user/                    resource de usuários (com os 3 modos de upload)
app/loom/admin/widgets/                 widgets do dashboard
packages/loom/                          o Loom
```

```bash
go run . artisan loom:make-user                  # criar um administrador
go run . artisan loom:make-resource Car -g       # criar um resource a partir da tabela cars
```
