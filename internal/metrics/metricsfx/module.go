package metricsfx

import (
	"github.com/ghobs91/lodestone/internal/metrics/queuemetrics"
	"github.com/ghobs91/lodestone/internal/metrics/torrentmetrics"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"queue",
		fx.Provide(
			queuemetrics.New,
			torrentmetrics.New,
		),
	)
}
