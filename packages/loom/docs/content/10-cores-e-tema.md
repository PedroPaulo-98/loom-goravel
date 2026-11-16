---
title: Cores e tema
section: Personalização
lead: Troque a cor principal, crie paletas para a sua marca e defina o modo claro ou escuro.
---

## Cores do painel

O painel usa seis cores com nomes de **função**, iguais às do Filament:

| Nome | Usada em | Padrão |
|---|---|---|
| `primary` | botões, links, item ativo do menu, badges `primary` | Amber |
| `gray` | fundos, bordas, textos secundários | Zinc |
| `danger` | erros, botões de excluir | Red |
| `success` | mensagens de sucesso, ✓ | Green |
| `warning` | avisos, badges `warning` | Amber |
| `info` | badges `info`, estatísticas | Blue |

## Trocando cores

No provider, passe só as cores que quer mudar:

```go
import "goravel/packages/loom/color"

Colors(map[string]color.Palette{
	"primary": color.Sky,
	"gray":    color.Slate,
})
```

### Paletas disponíveis

`color.Slate`, `color.Gray`, `color.Zinc`, `color.Red`, `color.Orange`, `color.Amber`, `color.Yellow`, `color.Lime`, `color.Green`, `color.Emerald`, `color.Teal`, `color.Cyan`, `color.Sky`, `color.Blue`, `color.Indigo`, `color.Violet`, `color.Purple`, `color.Fuchsia`, `color.Pink` e `color.Rose`.

São as paletas do Tailwind CSS v3, com as tonalidades de `50` a `950`.

## Adicionando mais cores

### Opção 1: direto no provider

Para uma cor usada só neste painel, monte a paleta ali mesmo. Uma paleta é um mapa de tonalidade para hex, com as 11 tonalidades:

```go
var marca = color.Palette{
	"50": "#eef7ff", "100": "#d9edff", "200": "#bce0ff", "300": "#8ecdff", "400": "#59b0ff",
	"500": "#338dfc", "600": "#1d6ef1", "700": "#1557de", "800": "#1847b4", "900": "#1a3f8e", "950": "#152856",
}

Colors(map[string]color.Palette{"primary": marca})
```

> Ferramentas como [uicolors.app](https://uicolors.app) geram as 11 tonalidades a partir de uma cor só.

### Opção 2: no pacote `color`

Para reutilizar em vários painéis, adicione a paleta em `packages/loom/color/color.go`, junto das outras:

```go
var (
	// ...
	Rose  = scale("#fff1f2", /* ... */ "#4c0519")
	Marca = scale("#eef7ff", "#d9edff", "#bce0ff", "#8ecdff", "#59b0ff", "#338dfc", "#1d6ef1", "#1557de", "#1847b4", "#1a3f8e", "#152856")
)
```

Depois é só usar `color.Marca` em qualquer provider. A função `scale` recebe os hex na ordem 50, 100, 200, ... 950.

### Opção 3: um nome de cor novo

Qualquer nome passado em `Colors` vira uma cor do Tailwind no painel:

```go
Colors(map[string]color.Palette{"secondary": color.Violet})
```

Assim, classes como `bg-secondary-500` e `text-secondary-700` passam a funcionar no HTML de [widgets](widgets) e [páginas](paginas) customizados.

> Os componentes prontos (badge, stats) só conhecem os seis nomes de função. Para usar uma cor nova num badge, troque uma das seis (ex.: `"info": color.Violet`).

## Modo claro e escuro

```go
DefaultThemeMode(loom.ThemeDark)   // escuro
DefaultThemeMode(loom.ThemeLight)  // claro
DefaultThemeMode(loom.ThemeSystem) // segue o sistema (padrão)
```

O usuário troca o tema pelo **menu do usuário**, que abre ao clicar no nome ou na foto no canto superior direito. A escolha fica salva no navegador e vale por cima do padrão do painel. Os gráficos acompanham o tema.

## Menu do usuário e acessibilidade

O menu do usuário reúne:

| Item | O que faz |
|---|---|
| **Tema** | Claro, Escuro ou Sistema. A opção ativa fica destacada. |
| **Tamanho da fonte** | `A−`, `Padrão` e `A+`, entre 85% e 150% (85, 100, 115, 130 e 150). |
| **Sair** | Encerra a sessão. |

Sobre o tamanho da fonte:

- O ajuste muda o tamanho base da página. Como todo o painel usa medidas em `rem`, aumentam juntos textos, espaçamentos, ícones e o texto dos gráficos.
- A escolha fica salva no navegador por painel, com a chave `loom-font-<id>`, e vale também nas telas de login.

O menu funciona pelo teclado: `Tab` chega ao botão, `Enter` abre e `Esc` fecha e devolve o foco ao botão. Clicar fora também fecha. Os botões informam o estado a leitores de tela (`aria-expanded`, `aria-pressed`) e anunciam o tamanho atual da fonte.

Para mudar o menu, edite o bloco `lm-user-menu` em `packages/loom/views/layout.html`.

## Largura do conteúdo

```go
MaxContentWidth("full")   // ocupa a tela toda
MaxContentWidth("7xl")    // padrão, ~80rem
```

## Fonte e CSS

O painel usa a fonte **Inter** e Tailwind CSS via CDN. Os templates ficam em `packages/loom/views/`:

| Arquivo | Conteúdo |
|---|---|
| `head.html` | `<head>`, tema, configuração do Tailwind, scripts |
| `layout.html` | menu lateral, topo e menu do usuário |
| `index.html` | listagem |
| `form.html` | formulário |
| `auth/*.html` | login, cadastro e recuperação de senha |
| `widgets/*.html` | stats, gráficos e cards |
| `emails/reset.html` | e-mail de redefinição de senha |

> Em produção, o ideal é compilar o Tailwind (com o CLI do Tailwind) em vez de usar o CDN.
