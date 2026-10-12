package concurrency

import (
	"context"
	"net/netip"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/time/rate"
)

type KeyedLimiter interface {
	Allow(key netip.Addr) bool
	Wait(ctx context.Context, key netip.Addr) error
}

type keyedLimiter struct {
	lru   *lru.LRU[netip.Addr, *rate.Limiter]
	mu    sync.RWMutex
	rl    rate.Limit
	burst int
}

func NewKeyedLimiter(rl rate.Limit, burst int, size int, ttl time.Duration) KeyedLimiter {
	i := &keyedLimiter{
		lru:   lru.NewLRU[netip.Addr, *rate.Limiter](size, nil, ttl),
		rl:    rl,
		burst: burst,
	}

	return i
}

func (i *keyedLimiter) Allow(key netip.Addr) bool {
	return i.getLimiter(key).Allow()
}

func (i *keyedLimiter) Wait(ctx context.Context, key netip.Addr) error {
	return i.getLimiter(key).Wait(ctx)
}

func (i *keyedLimiter) getLimiter(key netip.Addr) *rate.Limiter {
	// Fast path: try a read lock first for the common cache-hit case.
	i.mu.RLock()
	l, ok := i.lru.Get(key)
	i.mu.RUnlock()
	if ok {
		return l
	}

	// Slow path: write lock only when we need to insert a new limiter.
	i.mu.Lock()
	// Double-check after acquiring the write lock.
	if l, ok = i.lru.Get(key); ok {
		i.mu.Unlock()
		return l
	}
	l = rate.NewLimiter(i.rl, i.burst)
	i.lru.Add(key, l)
	i.mu.Unlock()

	return l
}
