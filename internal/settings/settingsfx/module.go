package settingsfx

import (
	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/lazy"
	"github.com/ghobs91/lodestone/internal/settings"
	"github.com/ghobs91/lodestone/internal/settings/httphandler"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"settings",
		fx.Provide(
			func(d lazy.Lazy[*dao.Query]) lazy.Lazy[settings.ClassifierSettingsStore] {
				return settings.NewClassifierSettingsStore(d)
			},
			httphandler.New,
		),
	)
}
