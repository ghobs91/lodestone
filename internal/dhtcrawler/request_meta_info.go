package dhtcrawler

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/ghobs91/lodestone/internal/protocol"
	"github.com/ghobs91/lodestone/internal/protocol/metainfo/metainforequester"
)

const (
	// maxParallelPeers is the number of peers to request metadata from concurrently.
	maxParallelPeers = 5
	// perPeerTimeout prevents a single slow peer from blocking the entire request.
	perPeerTimeout = 10 * time.Second
)

func (c *crawler) runRequestMetaInfo(ctx context.Context) {
	_ = c.requestMetaInfo.Run(ctx, func(req infoHashWithPeers) {
		mi, reqErr := c.doRequestMetaInfo(ctx, req.infoHash, req.peers)
		if reqErr != nil {
			return
		}
		select {
		case <-ctx.Done():
		case c.persistTorrents.In() <- infoHashWithMetaInfo{
			nodeHasPeersForHash: req.nodeHasPeersForHash,
			metaInfo:            mi.Info,
		}:
		}
	})
}

func (c *crawler) doRequestMetaInfo(
	ctx context.Context,
	hash protocol.ID,
	peers []netip.AddrPort,
) (metainforequester.Response, error) {
	// Race up to maxParallelPeers concurrently, accepting the first successful response.
	raceCtx, raceCancel := context.WithCancel(ctx)
	defer raceCancel()

	type result struct {
		resp metainforequester.Response
		err  error
	}

	resultCh := make(chan result, len(peers))

	var wg sync.WaitGroup

	sem := make(chan struct{}, maxParallelPeers)

	for _, p := range peers {
		select {
		case <-raceCtx.Done():
			break
		case sem <- struct{}{}:
		}

		wg.Add(1)

		go func(peer netip.AddrPort) {
			defer wg.Done()
			defer func() { <-sem }()

			peerCtx, peerCancel := context.WithTimeout(raceCtx, perPeerTimeout)
			defer peerCancel()

			res, err := c.metainfoRequester.Request(peerCtx, hash, peer)
			resultCh <- result{resp: res, err: err}
		}(p)
	}

	// Close resultCh when all goroutines complete.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var errs []error

	for r := range resultCh {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}

		if banErr := c.banningChecker.Check(r.resp.Info); banErr != nil {
			_ = c.blockingManager.Block(ctx, []protocol.ID{hash}, false)
			return metainforequester.Response{}, banErr
		}

		// First successful response wins; cancel remaining peers.
		raceCancel()

		return r.resp, nil
	}

	return metainforequester.Response{}, errors.Join(errs...)
}
