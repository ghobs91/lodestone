package resolvers

import (
	"github.com/ghobs91/lodestone/internal/blocking"
	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/database/search"
	"github.com/ghobs91/lodestone/internal/health"
	"github.com/ghobs91/lodestone/internal/metrics/queuemetrics"
	"github.com/ghobs91/lodestone/internal/metrics/torrentmetrics"
	"github.com/ghobs91/lodestone/internal/processor"
	"github.com/ghobs91/lodestone/internal/queue/manager"
	"github.com/ghobs91/lodestone/internal/worker"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Dao                  *dao.Query
	Search               search.Search
	Workers              worker.Registry
	Checker              health.Checker
	QueueMetricsClient   queuemetrics.Client
	QueueManager         manager.Manager
	TorrentMetricsClient torrentmetrics.Client
	Processor            processor.Processor
	BlockingManager      blocking.Manager
}
