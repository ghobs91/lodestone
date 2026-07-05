package dhtcrawler

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"github.com/ghobs91/lodestone/internal/blocking"
	"github.com/ghobs91/lodestone/internal/bloom"
	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/protocol"
	"github.com/ghobs91/lodestone/internal/protocol/dht/client"
	"github.com/ghobs91/lodestone/internal/protocol/dht/ktable"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/banning"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/metainforequester"
	"github.com/prometheus/client_golang/prometheus"
	boom "github.com/tylertreat/BoomFilters"
	"go.uber.org/zap"
)

type crawler struct {
	kTable                       ktable.Table
	client                       client.Client
	metainfoRequester            metainforequester.Requester
	banningChecker               banning.Checker
	bootstrapNodes               []string
	reseedBootstrapNodesInterval time.Duration
	getOldestNodesInterval       time.Duration
	oldPeerThreshold             time.Duration
	discoveredNodes              concurrency.BatchingChannel[ktable.Node]
	nodesForPing                 concurrency.BufferedConcurrentChannel[ktable.Node]
	nodesForFindNode             concurrency.BufferedConcurrentChannel[ktable.Node]
	nodesForSampleInfoHashes     concurrency.BufferedConcurrentChannel[ktable.Node]
	infoHashTriage               concurrency.BatchingChannel[nodeHasPeersForHash]
	getPeers                     concurrency.BufferedConcurrentChannel[nodeHasPeersForHash]
	scrape                       concurrency.BufferedConcurrentChannel[nodeHasPeersForHash]
	requestMetaInfo              concurrency.BufferedConcurrentChannel[infoHashWithPeers]
	persistTorrents              concurrency.BatchingChannel[infoHashWithMetaInfo]
	persistSources               concurrency.BatchingChannel[infoHashWithScrape]
	rescrapeThreshold            time.Duration
	saveFilesThreshold           uint
	savePieces                   bool
	dao                          *dao.Query
	// ignoreHashes is a thread-safe bloom filter that the crawler keeps in memory,
	// containing every hash it has already encountered.
	// This avoids multiple attempts to crawl the same hash, and takes a lot of load off the database query
	// that checks if a hash has already been indexed.
	ignoreHashes    *ignoreHashes
	blockingManager blocking.Manager
	// soughtNodeID is a random node ID used as the target for find_node and sample_infohashes requests.
	// It is rotated every 10 seconds.
	soughtNodeID   *concurrency.AtomicValue[protocol.ID]
	stopped        chan struct{}
	persistedTotal *prometheus.CounterVec
	logger         *zap.SugaredLogger
}

func (c *crawler) start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// start the various pipeline workers
	go c.rotateSoughtNodeID(ctx)
	go c.runDiscoveredNodes(ctx)
	go c.runPing(ctx)
	go c.runFindNode(ctx)
	go c.getNodesForFindNode(ctx)
	go c.runSampleInfoHashes(ctx)
	go c.getNodesForSampleInfoHashes(ctx)
	go c.runInfoHashTriage(ctx)
	go c.runGetPeers(ctx)
	go c.runRequestMetaInfo(ctx)
	go c.runScrape(ctx)
	go c.reseedBootstrapNodes(ctx)
	go c.runPersistTorrents(ctx)
	go c.runPersistSources(ctx)
	go c.getOldNodes(ctx)
	<-c.stopped
}

type nodeHasPeersForHash struct {
	infoHash protocol.ID
	node     netip.AddrPort
}

type infoHashWithMetaInfo struct {
	nodeHasPeersForHash
	metaInfo metainfo.Info
}

type infoHashWithPeers struct {
	nodeHasPeersForHash
	peers []netip.AddrPort
}

type infoHashWithScrape struct {
	nodeHasPeersForHash
	bfsd bloom.Filter
	bfpe bloom.Filter
}

const ignoreHashesShards = 16

// ignoreHashes is a sharded, rotating dual-bloom-filter set that tracks
// info hashes the crawler has already encountered. Sharding eliminates
// mutex contention on the hot path. The dual-filter design (active +
// previous) prevents the periodic "reset spike" where all previously-seen
// hashes would suddenly pass through to the database after a single-filter
// reset.
type ignoreHashes struct {
	shards [ignoreHashesShards]*ignoreHashShard
}

type ignoreHashShard struct {
	mu       sync.Mutex
	active   *boom.StableBloomFilter
	previous *boom.StableBloomFilter
	count    uint64
	capacity uint64
	fpRate   float64
}

func newIgnoreHashes(capacity uint64, fpRate float64) *ignoreHashes {
	ih := &ignoreHashes{}
	shardCap := capacity / ignoreHashesShards
	for i := range ignoreHashesShards {
		ih.shards[i] = &ignoreHashShard{
			active:   boom.NewStableBloomFilter(uint(shardCap), 2, fpRate),
			capacity: shardCap,
			fpRate:   fpRate,
		}
	}
	return ih
}

func (i *ignoreHashes) testAndAdd(id protocol.ID) bool {
	// Shard by the first byte to distribute contention.
	shard := i.shards[id[0]%ignoreHashesShards]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Check both filters: if present in either, it's a duplicate.
	if shard.active.TestAndAdd(id[:]) {
		return true
	}
	if shard.previous != nil && shard.previous.Test(id[:]) {
		return true
	}

	shard.count++

	// When the active filter is full, rotate: previous becomes the old
	// active, and we start a fresh active filter. Hashes in the previous
	// filter are still consulted for one full rotation cycle, smoothing
	// out the false-positive rate and preventing a sudden DB spike.
	if shard.count >= shard.capacity {
		shard.previous = shard.active
		shard.active = boom.NewStableBloomFilter(uint(shard.capacity), 2, shard.fpRate)
		shard.count = 0
	}

	return false
}

func (c *crawler) rotateSoughtNodeID(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
			c.soughtNodeID.Set(protocol.RandomNodeID())
		}
	}
}
