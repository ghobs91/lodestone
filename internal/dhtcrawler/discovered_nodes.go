package dhtcrawler

import (
	"context"
	"net/netip"
	"time"

	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/ghobs91/lodestone/internal/protocol/dht/ktable"
	"go.uber.org/fx"
)

type DiscoveredNodesParams struct {
	fx.In
	Config Config
}

type DiscoveredNodesResult struct {
	fx.Out
	DiscoveredNodes concurrency.BatchingChannel[ktable.Node] `name:"dht_discovered_nodes"`
}

// NewDiscoveredNodes creates the channel for discovered nodes.
// It receives nodes discovered by the crawler, as well as nodes from incoming requests to the DHT server.
// It is provided as a separate service to avoid a circular dependency with the DHT server.
func NewDiscoveredNodes(params DiscoveredNodesParams) DiscoveredNodesResult {
	return DiscoveredNodesResult{
		DiscoveredNodes: concurrency.NewBatchingChannel[ktable.Node](
			int(100*params.Config.ScalingFactor), 10, time.Second/10),
	}
}

func (c *crawler) runDiscoveredNodes(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ps := <-c.discoveredNodes.Out():
			addrs := make([]netip.Addr, 0, 1)

			m := make(map[string]ktable.Node, 1)
			for _, p := range ps {
				if !isRoutableAddr(p.Addr()) {
					c.discoveredAddrs.WithLabelValues("invalid").Inc()

					continue
				}

				if _, ok := m[p.Addr().Addr().String()]; !ok {
					m[p.Addr().Addr().String()] = p
					addrs = append(addrs, p.Addr().Addr())
				}
			}
			// for any discovered node not already in the routing table,
			// we will block until it can be sent to any one of the pipeline channels.
			unknownAddrs := c.kTable.FilterKnownAddrs(addrs)
			c.discoveredAddrs.WithLabelValues("known").Add(float64(len(addrs) - len(unknownAddrs)))
			c.discoveredAddrs.WithLabelValues("admitted").Add(float64(len(unknownAddrs)))
			for _, addr := range unknownAddrs {
				p := m[addr.String()]
				select {
				case <-ctx.Done():
					return
				case c.nodesForFindNode.In() <- p:
				case c.nodesForSampleInfoHashes.In() <- p:
				case c.nodesForPing.In() <- p:
				}
			}
		}
	}
}

// isRoutableAddr reports whether an address is a plausible public DHT peer.
// DHT responses are full of peers advertising loopback, RFC1918, CGNAT and
// link-local addresses, none of which can ever answer: probing them burns query
// budget (and, on a home network, conntrack entries) for nothing.
func isRoutableAddr(ap netip.AddrPort) bool {
	if ap.Port() == 0 {
		return false
	}

	addr := ap.Addr()

	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() ||
		addr.IsPrivate() || addr.IsMulticast() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() {
		return false
	}

	if addr.Is4() {
		b := addr.As4()
		// 100.64.0.0/10 carrier-grade NAT.
		if b[0] == 100 && b[1] >= 64 && b[1] <= 127 {
			return false
		}

		// 0.0.0.0/8 and 240.0.0.0/4 (reserved, incl. broadcast).
		if b[0] == 0 || b[0] >= 240 {
			return false
		}
	}

	return true
}
