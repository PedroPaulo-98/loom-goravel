package providers

import (
	"time"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/foundation"

	"goravel/app/facades"
	apiuser "goravel/app/loom/api/user"
	"goravel/packages/loom"
	"goravel/packages/loom/table"
)

// APIProvider configures the JSON API. It is registered in bootstrap/providers.go.
type APIProvider struct{}

// API configures the API.
func (r *APIProvider) API(a *loom.API) *loom.API {
	return a.
		Prefix("/api/v1").
		RateLimit(60).
		TokenLogin().
		TokenExpiry(30 * 24 * time.Hour).
		CanAccess(func(u table.Row) bool {
			return table.Boolean("is_active").Truthy(u)
		}).
		Resources(
			&apiuser.Resource{},
		)
}

func (r *APIProvider) Services() loom.Services {
	return loom.Services{
		Route:      facades.Route(),
		DB:         facades.DB(),
		Validation: facades.Validation(),
		Hash:       facades.Hash(),
	}
}

func (r *APIProvider) Register(app foundation.Application) {}

func (r *APIProvider) Boot(app foundation.Application) {
	a := r.API(loom.NewAPI())
	app.Commands([]console.Command{
		loom.NewMakeTokenCommand(a),
	})
	if err := a.Register(r.Services()); err != nil {
		panic(err)
	}
}
