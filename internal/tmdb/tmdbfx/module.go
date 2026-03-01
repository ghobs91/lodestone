package tmdbfx

import (
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/tmdb"
	"github.com/ghobs91/lodestone/internal/tmdb/tmdbhealthcheck"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"tmdb",
		configfx.NewConfigModule[tmdb.Config]("tmdb", tmdb.NewDefaultConfig()),
		fx.Provide(
			tmdb.New,
			tmdbhealthcheck.New,
		),
	)
}
