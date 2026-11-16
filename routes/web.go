package routes

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support"

	"goravel/app/facades"
	loomdocs "goravel/packages/loom/docs"
)

func Web() {
	facades.Route().Get("/", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("welcome.tmpl", map[string]any{
			"version": support.Version,
		})
	})

	facades.Route().Static("public", "./public")

	if _, err := loomdocs.Register(facades.Route(), "/loom/docs"); err != nil {
		panic(err)
	}
}
