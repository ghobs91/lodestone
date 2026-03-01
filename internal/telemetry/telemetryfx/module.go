package telemetryfx

import (
	"github.com/ghobs91/lodestone/internal/telemetry/httpserver"
	"github.com/ghobs91/lodestone/internal/telemetry/prometheus"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"telemetry",
		fx.Provide(
			httpserver.New,
			prometheus.New,
		),
	)
}
