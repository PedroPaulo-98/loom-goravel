---
title: Navegação
section: Painel
lead: Grupos, ordem, ícones e o menu lateral recolhível.
---

## Como o menu é montado

O menu lateral lista, nesta ordem:

1. Os itens **sem grupo**: o Dashboard e qualquer resource ou página sem `NavigationGroup`.
2. Os grupos, na ordem definida em `NavigationGroups(...)`.
3. Os grupos que não estão nessa lista, em ordem alfabética.

Dentro de cada grupo, os itens são ordenados por `NavigationSort()` (o menor primeiro) e, em caso de empate, pelo nome.

## Definindo os grupos

No provider:

```go
NavigationGroups(
	"Gestão de Pessoas",
	"Relatórios",
	"Solicitações",
	"Administração",
	"Sistema",
)
```

No resource (ou página):

```go
func (Resource) NavigationGroup() string { return "Administração" }
func (Resource) NavigationSort() int     { return 1 }
```

## Ícones

Cada item usa um ícone do [Heroicons](https://heroicons.com), pelo nome:

```go
func (Resource) Icon() string { return "users" }            // outline
func (Resource) Icon() string { return "heroicon-s-users" } // solid
```

Detalhes em [Ícones](icones).

## Menu recolhível

```go
SidebarCollapsibleOnDesktop()
```

Adiciona o botão **Recolher menu** no rodapé do menu lateral. Recolhido, o menu mostra só os ícones (o nome aparece ao passar o mouse). A escolha fica salva no navegador, separada por painel.

No celular, o menu vira uma barra horizontal no topo.

## Esconder uma página do menu

Uma página pode existir (e ser acessada pela URL) sem aparecer no menu:

```go
func (Relatorio) HiddenFromNavigation() bool { return true }
```
