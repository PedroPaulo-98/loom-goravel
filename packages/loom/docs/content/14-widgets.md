---
title: Widgets
section: Extensões
lead: Blocos do dashboard: cards de estatística, gráficos ou qualquer HTML.
---

## Registrando widgets

```go
Widgets(
	widgets.UsersOverview(),
	widgets.UsersByRole(),
	widgets.UsersPerMonth(),
)
```

Os widgets aparecem no Dashboard num grid de 3 colunas. Sem nenhum widget registrado, o Dashboard mostra um card com o total de registros de cada resource (`loom.ResourceCountsWidget`).

Uma boa organização é deixar os widgets em `app/loom/admin/widgets/`.

## Stats

Cards com um número grande, uma descrição e um ícone. Equivale ao `StatsOverviewWidget` do Filament:

```go
func UsersOverview() loom.Widget {
	return &loom.StatsOverviewWidget{
		Heading: "Resumo", // opcional
		Order:   1,
		Stats: func(ctx loom.Context) ([]loom.Stat, error) {
			total, err := ctx.DB().Table("users").Count()
			if err != nil {
				return nil, err
			}
			ativos, err := ctx.DB().Table("users").Where("is_active", true).Count()
			if err != nil {
				return nil, err
			}
			return []loom.Stat{
				{Label: "Usuários", Value: strconv.FormatInt(total, 10), Description: "no total", Icon: "users"},
				{Label: "Ativos", Value: strconv.FormatInt(ativos, 10), Description: "com acesso", Icon: "check-badge", Color: "success"},
			}, nil
		},
	}
}
```

| Campo do `Stat` | Descrição |
|---|---|
| `Label` | título do card |
| `Value` | o número (texto livre: `"1.234"`, `"R$ 50 mil"`) |
| `Description` | texto pequeno abaixo do número |
| `Icon` | Heroicon ao lado da descrição |
| `Color` | cor da descrição: `primary`, `success`, `danger`, `warning`, `info` ou `gray` |

Até 4 cards por linha. O widget de stats ocupa a largura toda.

## Gráficos

Gráficos com [Chart.js](https://www.chartjs.org), que acompanham o modo escuro:

```go
func UsersByRole() loom.Widget {
	return &loom.ChartWidget{
		Heading:     "Usuários por perfil",
		Description: "Distribuição atual",
		Type:        loom.ChartDoughnut,
		Span:        1,
		Order:       2,
		Data: func(ctx loom.Context) (loom.ChartData, error) {
			return loom.ChartData{
				Labels: []string{"Administradores", "Usuários"},
				Datasets: []loom.Dataset{
					{Label: "Total", Data: []float64{6, 21}},
				},
			}, nil
		},
	}
}
```

| Tipo | Constante |
|---|---|
| Barras | `loom.ChartBar` (padrão) |
| Linha | `loom.ChartLine` |
| Pizza | `loom.ChartPie` |
| Rosca | `loom.ChartDoughnut` |
| Polar | `loom.ChartPolar` |
| Radar | `loom.ChartRadar` |

- **Várias séries**: passe mais de um `Dataset`; a legenda aparece automaticamente.
- **Cores**: são automáticas. Para fixá-las, use `Dataset{Colors: []string{"#0ea5e9", "#f59e0b"}}`.
- **Tamanho**: `Span` é o número de colunas do grid (1, 2 ou 3).

## Ordem e largura

| Campo / método | Descrição |
|---|---|
| `Order` / `Sort() int` | Ordem no dashboard (menor primeiro). |
| `Span` / `ColumnSpan() int` | Colunas ocupadas: 1, 2 ou 3. |

Um exemplo de linha comum: um gráfico de rosca com `Span: 1` ao lado de um de barras com `Span: 2`.

## Widget totalmente customizado

Qualquer tipo com o método `Render` é um widget:

```go
type Boas struct{}

func (Boas) Sort() int       { return 0 }
func (Boas) ColumnSpan() int { return 3 }

func (Boas) Render(ctx loom.Context) (template.HTML, error) {
	nome := template.HTMLEscapeString(table.Stringify(ctx.User()["name"]))
	return template.HTML(`
		<div class="rounded-xl border border-gray-200 bg-white p-6 dark:border-white/10 dark:bg-gray-900">
			<h2 class="text-lg font-semibold">Olá, ` + nome + `!</h2>
			<p class="text-sm text-gray-500 dark:text-gray-400">Tenha um bom trabalho.</p>
		</div>`), nil
}
```

> Ao montar HTML na mão, **sempre escape** dados do banco ou do usuário (`template.HTMLEscapeString`), ou use `html/template`.

As classes do Tailwind e as cores do painel (`primary-*`, `gray-*`...) funcionam normalmente.

## O contexto

Todo widget e toda página recebem um `loom.Context`:

| Campo / método | Descrição |
|---|---|
| `ctx.HTTP` | a requisição do Goravel (`http.Context`) |
| `ctx.DB()` | o banco, já com o contexto da requisição |
| `ctx.User()` | a linha do usuário logado |
| `ctx.Services()` | os serviços do painel |
| `ctx.Panel` | o painel |
