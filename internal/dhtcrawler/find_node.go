package dhtcrawler

import (
	"context"
	"fmt"
	"time"

	"github.com/ghobs91/lodestone/internal/protocol/dht/ktable"
)

func (c *crawler) getNodesForFindNode(ctx context.Context) {
	for {
		peers := c.kTable.GetOldestNodes(time.Now().Add(-(5 * time.Second)), 10)
		for _, p := range peers {
			select {
			case <-ctx.Done():
				return
			case c.nodesForFindNode.In() <- p:
				continue
			}
		}

		<-time.After(time.Second)
	}
}

func (c *crawler) runFindNode(ctx context.Context) {
	_ = c.nodesForFindNode.Run(ctx, func(p ktable.Node) {
		res, err := c.client.FindNode(ctx, p.Addr(), c.soughtNodeID.Get())
		if err != nil {
			c.kTable.BatchCommand(ktable.DropNode{
				ID:     p.ID(),
				Reason: fmt.Errorf("find_node failed: %w", err),
			})
		} else {
			c.kTable.BatchCommand(ktable.PutNode{
				ID:      p.ID(),
				Addr:    p.Addr(),
				Options: []ktable.NodeOption{ktable.NodeResponded()},
			})
			// Try to add discovered nodes to the pipeline; drop nodes
			// if the channel is full rather than blocking the worker.
			for _, n := range res.Nodes {
				select {
				case <-ctx.Done():
					return
				case c.discoveredNodes.In() <- ktable.NewNode(n.ID, n.Addr):
				default:
					// Channel full; drop the node to avoid pipeline stall.
				}
			}
		}
	})
}
