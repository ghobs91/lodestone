package tmdbhealthcheck

import (
	"time"

	"github.com/ghobs91/lodestone/internal/health"
	"github.com/ghobs91/lodestone/internal/lazy"
	"github.com/ghobs91/lodestone/internal/tmdb"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	Config tmdb.Config
	Client lazy.Lazy[tmdb.Client]
}

type Result struct {
	fx.Out
	Option health.CheckerOption `group:"health_check_options"`
}

func New(p Params) Result {
	return Result{
		Option: health.WithPeriodicCheck(
			time.Minute*5,
			0,
			NewCheck(p.Config.Enabled, p.Client),
		),
	}
}
