package devfx

import (
	"github.com/ghobs91/lodestone/internal/app/cli"
	"github.com/ghobs91/lodestone/internal/app/cli/args"
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/database"
	"github.com/ghobs91/lodestone/internal/database/migrations"
	"github.com/ghobs91/lodestone/internal/database/postgres"
	"github.com/ghobs91/lodestone/internal/dev/app/cmd/gormcmd"
	"github.com/ghobs91/lodestone/internal/dev/app/cmd/migratecmd"
	"github.com/ghobs91/lodestone/internal/logging/loggingfx"
	"github.com/ghobs91/lodestone/internal/validation/validationfx"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"dev",
		configfx.NewConfigModule[postgres.Config]("postgres", postgres.NewDefaultConfig()),
		configfx.New(),
		loggingfx.New(),
		validationfx.New(),
		fx.Provide(args.New),
		fx.Provide(cli.New),
		fx.Provide(database.New),
		fx.Provide(migrations.New),
		fx.Provide(postgres.New),
		fx.Provide(gormcmd.New),
		fx.Provide(migratecmd.New),
	)
}
