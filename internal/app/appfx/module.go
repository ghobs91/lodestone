package appfx

import (
	"github.com/ghobs91/lodestone/internal/app/cli"
	"github.com/ghobs91/lodestone/internal/app/cli/args"
	"github.com/ghobs91/lodestone/internal/app/cli/hooks"
	"github.com/ghobs91/lodestone/internal/app/cmd/classifiercmd"
	"github.com/ghobs91/lodestone/internal/app/cmd/configcmd"
	"github.com/ghobs91/lodestone/internal/app/cmd/processcmd"
	"github.com/ghobs91/lodestone/internal/app/cmd/reprocesscmd"
	"github.com/ghobs91/lodestone/internal/app/cmd/workercmd"
	"github.com/ghobs91/lodestone/internal/blocking/blockingfx"
	"github.com/ghobs91/lodestone/internal/classifier/classifierfx"
	"github.com/ghobs91/lodestone/internal/config/configfx"
	"github.com/ghobs91/lodestone/internal/database/databasefx"
	"github.com/ghobs91/lodestone/internal/database/migrations"
	"github.com/ghobs91/lodestone/internal/dhtcrawler/dhtcrawlerfx"
	"github.com/ghobs91/lodestone/internal/gql/gqlfx"
	"github.com/ghobs91/lodestone/internal/health/healthfx"
	"github.com/ghobs91/lodestone/internal/httpserver/httpserverfx"
	"github.com/ghobs91/lodestone/internal/importer/importerfx"
	"github.com/ghobs91/lodestone/internal/logging/loggingfx"
	"github.com/ghobs91/lodestone/internal/metrics/metricsfx"
	"github.com/ghobs91/lodestone/internal/processor/processorfx"
	"github.com/ghobs91/lodestone/internal/protocol/dht/dhtfx"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/metainfofx"
	"github.com/ghobs91/lodestone/internal/queue/queuefx"
	"github.com/ghobs91/lodestone/internal/settings/settingsfx"
	"github.com/ghobs91/lodestone/internal/telemetry/telemetryfx"
	"github.com/ghobs91/lodestone/internal/tmdb/tmdbfx"
	"github.com/ghobs91/lodestone/internal/torznab/torznabfx"
	"github.com/ghobs91/lodestone/internal/validation/validationfx"
	"github.com/ghobs91/lodestone/internal/version/versionfx"
	"github.com/ghobs91/lodestone/internal/webui"
	"github.com/ghobs91/lodestone/internal/worker/workerfx"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"app",
		blockingfx.New(),
		classifierfx.New(),
		configfx.New(),
		dhtcrawlerfx.New(),
		dhtfx.New(),
		databasefx.New(),
		gqlfx.New(),
		healthfx.New(),
		httpserverfx.New(),
		importerfx.New(),
		loggingfx.New(),
		metainfofx.New(),
		metricsfx.New(),
		processorfx.New(),
		queuefx.New(),
		settingsfx.New(),
		telemetryfx.New(),
		tmdbfx.New(),
		torznabfx.New(),
		validationfx.New(),
		versionfx.New(),
		workerfx.New(),
		fx.Provide(
			args.New,
			cli.New,
			hooks.New,
			// cli commands:
			classifiercmd.New,
			configcmd.New,
			reprocesscmd.New,
			processcmd.New,
			workercmd.New,
		),
		fx.Provide(webui.New),
		fx.Decorate(migrations.NewDecorator),
	)
}
