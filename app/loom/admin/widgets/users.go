// Package widgets has the dashboard widgets of the admin panel.
package widgets

import (
	"strconv"
	"time"

	"goravel/packages/loom"
)

// UsersOverview shows total, active and new users.
func UsersOverview() loom.Widget {
	return &loom.StatsOverviewWidget{
		Order: 1,
		Stats: func(ctx loom.Context) ([]loom.Stat, error) {
			db := ctx.DB()
			total, err := db.Table("users").Count()
			if err != nil {
				return nil, err
			}
			active, err := db.Table("users").Where("is_active", true).Count()
			if err != nil {
				return nil, err
			}
			admins, err := db.Table("users").Where("role", "admin").Count()
			if err != nil {
				return nil, err
			}
			recent, err := db.Table("users").Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count()
			if err != nil {
				return nil, err
			}
			return []loom.Stat{
				{Label: "Usuários", Value: itoa(total), Description: "cadastrados no total", Icon: "users"},
				{Label: "Ativos", Value: itoa(active), Description: itoa(total-active) + " inativos", Icon: "check-badge", Color: "success"},
				{Label: "Administradores", Value: itoa(admins), Description: "com acesso ao painel", Icon: "shield-check", Color: "primary"},
				{Label: "Novos (7 dias)", Value: itoa(recent), Description: "últimos 7 dias", Icon: "arrow-trending-up", Color: "info"},
			}, nil
		},
	}
}

// UsersByRole is a doughnut chart of users per role.
func UsersByRole() loom.Widget {
	return &loom.ChartWidget{
		Heading: "Usuários por perfil",
		Type:    loom.ChartDoughnut,
		Order:   2,
		Data: func(ctx loom.Context) (loom.ChartData, error) {
			db := ctx.DB()
			admins, err := db.Table("users").Where("role", "admin").Count()
			if err != nil {
				return loom.ChartData{}, err
			}
			users, err := db.Table("users").Where("role", "user").Count()
			if err != nil {
				return loom.ChartData{}, err
			}
			return loom.ChartData{
				Labels:   []string{"Administradores", "Usuários"},
				Datasets: []loom.Dataset{{Label: "Usuários", Data: []float64{float64(admins), float64(users)}}},
			}, nil
		},
	}
}

// UsersPerMonth is a bar chart of sign-ups in the last 6 months.
func UsersPerMonth() loom.Widget {
	return &loom.ChartWidget{
		Heading:     "Cadastros por mês",
		Description: "Últimos 6 meses",
		Type:        loom.ChartBar,
		Span:        2,
		Order:       3,
		Data: func(ctx loom.Context) (loom.ChartData, error) {
			now := time.Now()
			start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -5, 0)
			var labels []string
			var values []float64
			for i := 0; i < 6; i++ {
				from := start.AddDate(0, i, 0)
				to := from.AddDate(0, 1, 0)
				n, err := ctx.DB().Table("users").
					Where("created_at >= ?", from).
					Where("created_at < ?", to).
					Count()
				if err != nil {
					return loom.ChartData{}, err
				}
				labels = append(labels, from.Format("01/2006"))
				values = append(values, float64(n))
			}
			return loom.ChartData{
				Labels:   labels,
				Datasets: []loom.Dataset{{Label: "Novos usuários", Data: values}},
			}, nil
		},
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
