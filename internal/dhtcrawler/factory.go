package dhtcrawler

import (
	"context"
	"time"

	"github.com/ghobs91/lodestone/internal/blocking"
	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/database/search"
	"github.com/ghobs91/lodestone/internal/lazy"
	"github.com/ghobs91/lodestone/internal/protocol"
	"github.com/ghobs91/lodestone/internal/protocol/dht/client"
	"github.com/ghobs91/lodestone/internal/protocol/dht/ktable"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/banning"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/metainforequester"
	"github.com/ghobs91/lodestone/internal/worker"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Params struct {
	fx.In
	Config            Config
	KTable            ktable.Table
	Client            lazy.Lazy[client.Client]
	MetainfoRequester metainforequester.Requester
	BanningChecker    banning.Checker `name:"metainfo_banning_checker"`
	Search            lazy.Lazy[search.Search]
	Dao               lazy.Lazy[*dao.Query]
	BlockingManager   lazy.Lazy[blocking.Manager]
	DiscoveredNodes   concurrency.BatchingChannel[ktable.Node] `name:"dht_discovered_nodes"`
	Logger            *zap.SugaredLogger
}

type Result struct {
	fx.Out
	Worker worker.Worker `group:"workers"`

	DhtCrawlerActive *concurrency.AtomicValue[bool] `name:"dht_crawler_active"`

	PersistedTotal prometheus.Collector `group:"prometheus_collectors"`
}

func New(params Params) Result {
	active := &concurrency.AtomicValue[bool]{}

	var c crawler

	persistedTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "lodestone",
		Subsystem: "dht_crawler",
		Name:      "persisted_total",
		Help:      "A counter of persisted database entities.",
	}, []string{"entity"})

	return Result{
		Worker: worker.NewWorker(
			"dht_crawler",
			fx.Hook{
				OnStart: func(context.Context) error {
					active.Set(true)
					scalingFactor := int(params.Config.ScalingFactor)
					cl, err := params.Client.Get()
					if err != nil {
						return err
					}
					query, err := params.Dao.Get()
					if err != nil {
						return err
					}
					blockingManager, err := params.BlockingManager.Get()
					if err != nil {
						return err
					}
					c = crawler{
						kTable:                       params.KTable,
						client:                       cl,
						metainfoRequester:            params.MetainfoRequester,
						banningChecker:               params.BanningChecker,
						bootstrapNodes:               params.Config.BootstrapNodes,
						reseedBootstrapNodesInterval: time.Minute * 10,
						getOldestNodesInterval:       time.Second * 10,
						oldPeerThreshold:             time.Minute * 15,
						discoveredNodes:              params.DiscoveredNodes,
						nodesForPing: concurrency.NewBufferedConcurrentChannel[ktable.Node](
							scalingFactor, scalingFactor),
						nodesForFindNode: concurrency.NewBufferedConcurrentChannel[ktable.Node](
							10*scalingFactor, 10*scalingFactor),
						nodesForSampleInfoHashes: concurrency.NewBufferedConcurrentChannel[ktable.Node](
							10*scalingFactor,
							10*scalingFactor,
						),
						infoHashTriage: concurrency.NewBatchingChannel[nodeHasPeersForHash](
							10*scalingFactor, 1000, 20*time.Second),
						getPeers: concurrency.NewBufferedConcurrentChannel[nodeHasPeersForHash](
							10*scalingFactor, 20*scalingFactor),
						scrape: concurrency.NewBufferedConcurrentChannel[nodeHasPeersForHash](
							10*scalingFactor, 20*scalingFactor),
						requestMetaInfo: concurrency.NewBufferedConcurrentChannel[infoHashWithPeers](
							10*scalingFactor,
							40*scalingFactor,
						),
						persistTorrents: concurrency.NewBatchingChannel[infoHashWithMetaInfo](
							1000,
							1000,
							time.Minute,
						),
						persistSources: concurrency.NewBatchingChannel[infoHashWithScrape](
							1000,
							1000,
							time.Minute,
						),
						saveFilesThreshold: params.Config.SaveFilesThreshold,
						savePieces:         params.Config.SavePieces,
						rescrapeThreshold:  params.Config.RescrapeThreshold,
						dao:                query,
						ignoreHashes: newIgnoreHashes(10_000_000, 0.001),
						blockingManager: blockingManager,
						soughtNodeID:    &concurrency.AtomicValue[protocol.ID]{},
						stopped:         make(chan struct{}),
						persistedTotal:  persistedTotal,
						logger:          params.Logger.Named("dht_crawler"),
					}
					c.soughtNodeID.Set(protocol.RandomNodeID())

					// todo: Fix!
					//nolint:contextcheck
					go c.start()
					return nil
				},
				OnStop: func(context.Context) error {
					active.Set(false)
					if c.stopped != nil {
						close(c.stopped)
					}
					return nil
				},
			},
		),
		PersistedTotal:   persistedTotal,
		DhtCrawlerActive: active,
	}
}
