package classifierfx

import (
	"github.com/ghobs91/lodestone/internal/classifier"
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"workflow",
		configfx.NewConfigModule[classifier.Config]("classifier", classifier.NewDefaultConfig()),
		fx.Provide(
			classifier.New,
		),
	)
}
