package torznabfx

import (
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/database/search"
	"github.com/ghobs91/lodestone/internal/lazy"
	"github.com/ghobs91/lodestone/internal/torznab"
	"github.com/ghobs91/lodestone/internal/torznab/adapter"
	"github.com/ghobs91/lodestone/internal/torznab/httpserver"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"torznab",
		configfx.NewConfigModule[torznab.Config]("torznab", torznab.NewDefaultConfig()),
		fx.Provide(
			func(lazySearch lazy.Lazy[search.Search]) lazy.Lazy[torznab.Client] {
				return lazy.New[torznab.Client](func() (torznab.Client, error) {
					s, err := lazySearch.Get()
					if err != nil {
						return nil, err
					}

					return adapter.New(s), nil
				})
			},
			fx.Annotate(
				httpserver.New,
				fx.ResultTags(`group:"http_server_options"`),
			),
		),
		fx.Decorate(
			func(cfg torznab.Config) torznab.Config {
				return cfg.MergeDefaults()
			}),
	)
}
