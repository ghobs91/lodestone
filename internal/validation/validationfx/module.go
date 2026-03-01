package validationfx

import (
	"github.com/ghobs91/lodestone/internal/validation"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"validation",
		fx.Provide(validation.New),
	)
}
