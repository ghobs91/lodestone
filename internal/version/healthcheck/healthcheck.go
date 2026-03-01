package healthcheck

import (
	"github.com/ghobs91/lodestone/internal/health"
	"github.com/ghobs91/lodestone/internal/version"
	"go.uber.org/fx"
)

type Result struct {
	fx.Out
	HealthOption health.CheckerOption `group:"health_check_options"`
}

func New() Result {
	return Result{
		HealthOption: health.WithInfo(map[string]any{
			"name":    "lodestone",
			"version": version.GitTag,
		}),
	}
}
