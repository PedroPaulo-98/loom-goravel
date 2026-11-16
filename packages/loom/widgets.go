package loom

import (
	"encoding/json"
	"html/template"
)

// Stat is one card of a StatsOverviewWidget.
type Stat struct {
	Label       string
	Value       string
	Description string
	// Icon is a Heroicon name shown next to the description.
	Icon string
	// Color of the description: primary, success, danger, warning, info, gray.
	Color string
}

// StatsOverviewWidget renders a row of stat cards (like Filament's StatsOverviewWidget).
//
//	&loom.StatsOverviewWidget{Stats: func(ctx loom.Context) ([]loom.Stat, error) { ... }}
type StatsOverviewWidget struct {
	Heading string
	Stats   func(ctx Context) ([]Stat, error)
	Order   int
}

func (w *StatsOverviewWidget) Sort() int       { return w.Order }
func (w *StatsOverviewWidget) ColumnSpan() int { return 3 }

func (w *StatsOverviewWidget) Render(ctx Context) (template.HTML, error) {
	stats, err := w.Stats(ctx)
	if err != nil {
		return "", err
	}
	return ctx.Panel.render.fragment("widget-stats", map[string]any{"Heading": w.Heading, "Stats": stats})
}

// ChartType is a Chart.js chart type.
type ChartType string

const (
	ChartBar      ChartType = "bar"
	ChartLine     ChartType = "line"
	ChartPie      ChartType = "pie"
	ChartDoughnut ChartType = "doughnut"
	ChartPolar    ChartType = "polarArea"
	ChartRadar    ChartType = "radar"
)

// ChartData is the data of a ChartWidget.
type ChartData struct {
	Labels   []string  `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

// Dataset is a series of a chart. Colors are optional (a default palette is used).
type Dataset struct {
	Label  string    `json:"label"`
	Data   []float64 `json:"data"`
	Colors []string  `json:"backgroundColor,omitempty"`
}

// ChartWidget renders a Chart.js chart (like Filament's ChartWidget).
type ChartWidget struct {
	Heading     string
	Description string
	Type        ChartType
	Data        func(ctx Context) (ChartData, error)
	// Span is the number of grid columns (1-3). Default 1.
	Span  int
	Order int
}

func (w *ChartWidget) Sort() int { return w.Order }
func (w *ChartWidget) ColumnSpan() int {
	if w.Span == 0 {
		return 1
	}
	return w.Span
}

var chartPalette = []string{"#0ea5e9", "#f59e0b", "#10b981", "#8b5cf6", "#ef4444", "#14b8a6", "#ec4899", "#6366f1", "#84cc16", "#f97316"}

func (w *ChartWidget) Render(ctx Context) (template.HTML, error) {
	data, err := w.Data(ctx)
	if err != nil {
		return "", err
	}
	kind := w.Type
	if kind == "" {
		kind = ChartBar
	}
	perPoint := kind == ChartPie || kind == ChartDoughnut || kind == ChartPolar
	for i := range data.Datasets {
		ds := &data.Datasets[i]
		if len(ds.Colors) > 0 {
			continue
		}
		if perPoint {
			for j := range ds.Data {
				ds.Colors = append(ds.Colors, chartPalette[j%len(chartPalette)])
			}
		} else {
			ds.Colors = []string{chartPalette[i%len(chartPalette)]}
		}
	}
	config, err := json.Marshal(map[string]any{"type": kind, "data": data})
	if err != nil {
		return "", err
	}
	return ctx.Panel.render.fragment("widget-chart", map[string]any{
		"Heading":     w.Heading,
		"Description": w.Description,
		"Config":      string(config),
		"Round":       perPoint,
	})
}

// ResourceCountsWidget shows one card per resource with its number of records.
type ResourceCountsWidget struct{ Order int }

func (w *ResourceCountsWidget) Sort() int       { return w.Order }
func (w *ResourceCountsWidget) ColumnSpan() int { return 3 }

func (w *ResourceCountsWidget) Render(ctx Context) (template.HTML, error) {
	type card struct {
		Label, URL, Icon string
		Count            int64
	}
	p := ctx.Panel
	cards := make([]card, 0, len(p.resources))
	for _, r := range p.resources {
		count, err := p.svc.DB.WithContext(ctx.HTTP).Table(r.TableName()).Count()
		if err != nil {
			return "", err
		}
		cards = append(cards, card{Label: r.PluralLabel(), URL: p.URL(r.Slug()), Icon: r.IconName(), Count: count})
	}
	return p.render.fragment("widget-counts", map[string]any{"Cards": cards})
}
