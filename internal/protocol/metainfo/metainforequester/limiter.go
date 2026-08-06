package metainforequester

import (
	"context"
	"net/netip"

	"github.com/ghobs91/lodestone/internal/concurrency"
	"github.com/ghobs91/lodestone/internal/protocol"
	"golang.org/x/time/rate"
)

type requestLimiter struct {
	requester     Requester
	limiter       concurrency.KeyedLimiter
	globalLimiter *rate.Limiter
}

func (r requestLimiter) Request(ctx context.Context, infoHash protocol.ID, node netip.AddrPort) (Response, error) {
	// Enforce global rate limit first (if configured) to cap total outbound
	// metadata requests regardless of how many distinct peers we talk to.
	if r.globalLimiter != nil {
		if limitErr := r.globalLimiter.Wait(ctx); limitErr != nil {
			return Response{}, limitErr
		}
	}

	if limitErr := r.limiter.Wait(ctx, node.Addr().String()); limitErr != nil {
		return Response{}, limitErr
	}

	return r.requester.Request(ctx, infoHash, node)
}
