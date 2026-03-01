package queuefx

import (
	"github.com/ghobs91/lodestone/internal/queue/manager"
	"github.com/ghobs91/lodestone/internal/queue/prometheus"
	"github.com/ghobs91/lodestone/internal/queue/server"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"queue",
		fx.Provide(
			server.New,
			manager.New,
			prometheus.New,
		),
	)
}
