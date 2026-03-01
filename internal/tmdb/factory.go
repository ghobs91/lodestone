package tmdb

import (
	"github.com/ghobs91/lodestone/internal/lazy"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Params struct {
	fx.In
	Config Config
	Logger *zap.SugaredLogger
}

type Result struct {
	fx.Out
	Client lazy.Lazy[Client]
}

func New(p Params) Result {
	return Result{
		Client: lazy.New(func() (Client, error) {
			return newCachedClient(client{
				requester: &requesterLazy{
					config: p.Config,
					logger: p.Logger,
				},
			}), nil
		}),
	}
}
