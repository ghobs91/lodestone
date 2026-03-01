package databasefx

import (
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/database"
	"github.com/ghobs91/lodestone/internal/database/cache"
	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/database/healthcheck"
	"github.com/ghobs91/lodestone/internal/database/migrations"
	"github.com/ghobs91/lodestone/internal/database/postgres"
	"github.com/ghobs91/lodestone/internal/database/search"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"database",
		configfx.NewConfigModule[postgres.Config]("postgres", postgres.NewDefaultConfig()),
		configfx.NewConfigModule[cache.Config]("gorm_cache", cache.NewDefaultConfig()),
		fx.Provide(
			cache.NewInMemoryCacher,
			cache.NewPlugin,
			dao.New,
			database.New,
			healthcheck.New,
			migrations.New,
			postgres.New,
			search.New,
		),
		fx.Decorate(
			cache.NewDecorator,
		),
	)
}
