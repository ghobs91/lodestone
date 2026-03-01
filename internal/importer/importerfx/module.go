package importerfx

import (
	"github.com/ghobs91/lodestone/internal/importer"
	"github.com/ghobs91/lodestone/internal/importer/httpserver"
	"go.uber.org/fx"
)

func New() fx.Option {
	return fx.Module(
		"importer",
		fx.Provide(
			httpserver.New,
			httpserver.NewSqliteImport,
			importer.New,
		),
	)
}
