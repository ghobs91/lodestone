package metainfofx

import (
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/banning"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/metainforequester"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"metainfo",
		configfx.NewConfigModule[metainforequester.Config](
			"metainfo_requester",
			metainforequester.NewDefaultConfig(),
		),
		fx.Provide(
			metainforequester.New,
			banning.New,
		),
	)
}
