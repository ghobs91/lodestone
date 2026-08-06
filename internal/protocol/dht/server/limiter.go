package server

import (
	"context"
	"net/netip"

	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/ghobs91/lodestone/internal/protocol/dht"
	"golang.org/x/time/rate"
)

type queryLimiter struct {
	server        Server
	queryLimiter  concurrency.KeyedLimiter
	globalLimiter *rate.Limiter
}

func (s queryLimiter) start() error {
	return s.server.start()
}

func (s queryLimiter) stop() {
	s.server.stop()
}

func (s queryLimiter) Query(
	ctx context.Context,
	addr netip.AddrPort,
	q string,
	args dht.MsgArgs,
) (r dht.RecvMsg, err error) {
	// Enforce global rate limit first (if configured) to cap total outbound
	// DHT queries regardless of how many distinct peers we are talking to.
	if s.globalLimiter != nil {
		if limitErr := s.globalLimiter.Wait(ctx); limitErr != nil {
			return r, limitErr
		}
	}

	if limitErr := s.queryLimiter.Wait(ctx, addr.Addr().String()); limitErr != nil {
		return r, limitErr
	}

	return s.server.Query(ctx, addr, q, args)
}
