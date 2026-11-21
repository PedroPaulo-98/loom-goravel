// Package providers holds the application service providers.
package providers

import (
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/foundation"

	"goravel/app/facades"
	"goravel/app/loom/admin/user"
	"goravel/app/loom/admin/widgets"
	"goravel/packages/loom"
	"goravel/packages/loom/color"
	"goravel/packages/loom/table"
)

// AdminPanelProvider configures the "admin" panel. It is registered in bootstrap/providers.go.
type AdminPanelProvider struct{}

// Panel configures the panel.
func (r *AdminPanelProvider) Panel(panel *loom.Panel) *loom.Panel {
	return panel.
		ID("admin").
		Path("/admin").
		BrandName(facades.Config().GetString("app.name", "Admin")).
		BaseURL(facades.Config().GetString("app.url")).
		DefaultThemeMode(loom.ThemeSystem).
		MaxContentWidth("full").
		Colors(map[string]color.Palette{
			"primary": color.Sky,
		}).
		NavigationGroups(
			"Administração",
			"Sistema",
		).
		Registration(map[string]any{"role": "user", "is_active": true}).
		PasswordReset().
		CanAccess(func(u table.Row) bool {
			return table.Stringify(u["role"]) == "admin" && table.Boolean("is_active").Truthy(u)
		}).
		Resources(
			&user.Resource{},
			&loom.APITokenResource{Group: "Sistema"},
		).
		Widgets(
			widgets.UsersOverview(),
			widgets.UsersByRole(),
			widgets.UsersPerMonth(),
		).
		SidebarCollapsibleOnDesktop()
}

// Services are the Goravel services the panel uses.
func (r *AdminPanelProvider) Services() loom.Services {
	return loom.Services{
		Route:      facades.Route(),
		DB:         facades.DB(),
		Validation: facades.Validation(),
		Hash:       facades.Hash(),
		Mail:       facades.Mail(),
		Storage:    facades.Storage(),
	}
}

func (r *AdminPanelProvider) Register(app foundation.Application) {}

func (r *AdminPanelProvider) Boot(app foundation.Application) {
	app.Commands([]console.Command{
		loom.NewMakeUserCommand(r.Services, map[string]any{"role": "admin", "is_active": true}),
		loom.NewMakeResourceCommand(loom.MakeResourceConfig{
			Dir:         "app/loom/admin",
			Provider:    "app/providers/admin_panel_provider.go",
			Schema:      facades.Schema,
			APIDir:      "app/loom/api",
			APIProvider: "app/providers/api_provider.go",
		}),
	})

	if err := r.Panel(loom.NewPanel()).Register(r.Services()); err != nil {
		panic(err)
	}
}
