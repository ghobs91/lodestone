package ktable

import (
	"net/netip"
	"time"

	"github.com/ghobs91/lodestone/internal/protocol/dht/ktable/btree"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	NodeID ID `name:"dht_node_id"`
}

type Result struct {
	fx.Out
	Table                Table
	NodesCountGauge      prometheus.Collector `group:"prometheus_collectors"`
	NodesAddedCounter    prometheus.Collector `group:"prometheus_collectors"`
	NodesDroppedCounter  prometheus.Collector `group:"prometheus_collectors"`
	HashesCountGauge     prometheus.Collector `group:"prometheus_collectors"`
	HashesAddedCounter   prometheus.Collector `group:"prometheus_collectors"`
	HashesDroppedCounter prometheus.Collector `group:"prometheus_collectors"`
	FailedAddrsGauge     prometheus.Collector `group:"prometheus_collectors"`
}

const (
	nodesK  = 80
	hashesK = 80

	// failedAddrCacheSize bounds the negative address cache. At the observed
	// discovery rate (~300 candidates/s) a 15 minute TTL would otherwise hold
	// millions of entries.
	failedAddrCacheSize = 100_000
	// failedAddrTTL is how long a failed address is kept out of node discovery.
	failedAddrTTL = 15 * time.Minute
)

func New(p Params) Result {
	rm := &reverseMap{addrs: make(map[string]*infoForAddr)}
	nodes := nodeKeyspace{
		keyspace: newKeyspace[netip.AddrPort, NodeOption, Node, *node](
			p.NodeID,
			nodesK,
			func(id ID, addr netip.AddrPort) *node {
				return &node{
					nodeBase: nodeBase{
						id:   id,
						addr: addr,
					},
					discoveredAt: time.Now(),
					reverseMap:   rm,
				}
			},
		),
	}
	nodesCollector := patchPrometheusCollector("nodes", &nodes.keyspace)
	hashes := hashKeyspace{
		keyspace: newKeyspace[[]HashPeer, HashOption, Hash, *hash](
			p.NodeID,
			hashesK,
			func(id ID, peers []HashPeer) *hash {
				peersMap := make(map[string]HashPeer, len(peers))
				for _, p := range peers {
					peersMap[p.Addr.Addr().String()] = p
					rm.putAddrHashes(p.Addr.Addr(), id)
				}
				return &hash{
					id:           id,
					peers:        peersMap,
					discoveredAt: time.Now(),
					reverseMap:   rm,
				}
			},
		),
	}
	hashesCollector := patchPrometheusCollector("hashes", &hashes.keyspace)

	// If this gauge sits at failedAddrCacheSize the negative cache is thrashing
	// and dead addresses are being retried before their TTL expires.
	failedAddrs := expirable.NewLRU[netip.Addr, struct{}](failedAddrCacheSize, nil, failedAddrTTL)
	failedAddrsGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "failed_addrs_count",
		Help:      "Number of recently-failed addresses excluded from node discovery.",
	}, func() float64 {
		return float64(failedAddrs.Len())
	})

	return Result{
		Table: &table{
			origin:      p.NodeID,
			nodesK:      nodesK,
			hashesK:     hashesK,
			nodes:       nodes,
			hashes:      hashes,
			addrs:       rm,
			failedAddrs: failedAddrs,
		},
		NodesCountGauge:      nodesCollector.CountGauge,
		NodesAddedCounter:    nodesCollector.AddedCounter,
		NodesDroppedCounter:  nodesCollector.DroppedCounter,
		HashesCountGauge:     hashesCollector.CountGauge,
		HashesAddedCounter:   hashesCollector.AddedCounter,
		HashesDroppedCounter: hashesCollector.DroppedCounter,
		FailedAddrsGauge:     failedAddrsGauge,
	}
}

const (
	namespace = "lodestone"
	subsystem = "dht_ktable"
)

func patchPrometheusCollector[
	Input any,
	Option any,
	ItemPublic keyspaceItem,
	ItemPrivate keyspaceItemPrivate[Input, Option, ItemPublic],
](itemName string, ks *keyspace[Input, Option, ItemPublic, ItemPrivate]) btree.PrometheusCollector {
	collector := btree.PrometheusCollector{
		Btree: ks.btree,
		CountGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      itemName + "_count",
			Help:      "Number of " + itemName + " in routing table.",
		}),
		AddedCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      itemName + "_added",
			Help:      "Total number of " + itemName + " added to routing table.",
		}),
		DroppedCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      itemName + "_dropped",
			Help:      "Total number of " + itemName + " dropped from routing table.",
		}),
	}
	ks.btree = collector

	return collector
}
