package httpserverfx

import (
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/httpserver"
	"github.com/ghobs91/lodestone/internal/httpserver/cors"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"http_server",
		configfx.NewConfigModule[httpserver.Config]("http_server", httpserver.NewDefaultConfig()),
		fx.Provide(
			httpserver.New,
			cors.New,
		),
	)
}
