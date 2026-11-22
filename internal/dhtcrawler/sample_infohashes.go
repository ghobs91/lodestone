package dhtcrawler

import (
	"context"
	"errors"
	"time"

	"github.com/ghobs91/lodestone/internal/protocol/dht"
	"github.com/ghobs91/lodestone/internal/protocol/dht/ktable"
)

func (c *crawler) getNodesForSampleInfoHashes(ctx context.Context) {
	for {
		peers := c.kTable.GetNodesForSampleInfoHashes(60)
		for _, p := range peers {
			select {
			case <-ctx.Done():
				return
			case c.nodesForSampleInfoHashes.In() <- p:
				continue
			default:
				// Channel full; drop rather than forcing the send and
				// permanently saturating the global query rate limiter.
				continue
			}
		}

		<-time.After(time.Second)
	}
}

func (c *crawler) runSampleInfoHashes(ctx context.Context) {
	_ = c.nodesForSampleInfoHashes.Run(ctx, func(n ktable.Node) {
		if !n.IsSampleInfoHashesCandidate() {
			return
		}

		res, err := c.client.SampleInfoHashes(ctx, n.Addr(), c.soughtNodeID.Get())
		if err != nil {
			var dhtErr *dht.Error

			opts := []ktable.NodeOption{
				ktable.NodeSampleInfoHashesFailed(time.Now().Add(sampleFailureBackoff)),
			}

			if errors.As(err, &dhtErr) {
				// The node answered with a protocol error, so it is alive; it just
				// doesn't implement BEP 51. Remember that and keep using it for
				// find_node/get_peers instead of asking it again and again.
				c.sampleFailures.WithLabelValues("unsupported").Inc()
				opts = append(opts, ktable.NodeResponded(), ktable.NodeBep51Support(false))
			} else {
				// No response. The node may be dead, or it may answer other queries
				// but ignore sample requests; either way it should not keep consuming
				// the sample budget.
				c.sampleFailures.WithLabelValues("unreachable").Inc()
				c.kTable.FailAddr(n.Addr().Addr())
			}

			c.kTable.BatchCommand(ktable.PutNode{ID: n.ID(), Addr: n.Addr(), Options: opts})

			return
		}

		var discoveredHashes []nodeHasPeersForHash

		for _, s := range res.Samples {
			if !c.ignoreHashes.testAndAdd(s) {
				discoveredHashes = append(discoveredHashes, nodeHasPeersForHash{
					infoHash: s,
					node:     n.Addr(),
				})
			}
		}

		for _, h := range discoveredHashes {
			select {
			case <-ctx.Done():
				return
			case c.infoHashTriage.In() <- h:
			default:
				// Channel full; drop hash to avoid blocking the worker.
				// It will be re-discovered by future sample_infohashes calls.
			}
		}

		interval := res.Interval
		// most nodes request a 6 hour backoff time(!)
		// if we're still discovering info hashes from them then let's set a respectful interval instead
		if len(discoveredHashes) > 0 && interval > 300 {
			interval = 60
		}

		c.kTable.BatchCommand(ktable.PutNode{ID: n.ID(), Addr: n.Addr(), Options: []ktable.NodeOption{
			ktable.NodeResponded(),
			ktable.NodeBep51Support(true),
			ktable.NodeSampleInfoHashesRes(
				len(discoveredHashes),
				res.Num,
				time.Now().Add(time.Duration(interval)*time.Second),
			),
		}})

		if len(res.Nodes) > 0 {
			// block on the channel for up to a second trying to add sampled nodes to the discoveredNodes
			// channel
			go func() {
				timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()

				for _, n := range res.Nodes {
					select {
					case <-timeoutCtx.Done():
						return
					case c.discoveredNodes.In() <- ktable.NewNode(n.ID, n.Addr):
						continue
					}
				}
			}()
		}
	})
}
