package processorfx

import (
	"github.com/ghobs91/lodestone/internal/processor"
	batchqueue "github.com/ghobs91/lodestone/internal/processor/batch/queue"
	processorqueue "github.com/ghobs91/lodestone/internal/processor/queue"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"processor",
		fx.Provide(
			processor.New,
			processorqueue.New,
			batchqueue.New,
		),
	)
}
