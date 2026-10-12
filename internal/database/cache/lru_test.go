package cache

import (
	"context"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
	caches "github.com/mgdigital/gorm-cache/v2"
	"go.uber.org/zap"
)

type testItem struct {
	Name string
	Tags []string
}

func newTestCacher() *inMemoryCacher {
	return &inMemoryCacher{
		lru:    lru.NewLRU[string, *caches.Query](10, nil, time.Minute),
		logger: zap.NewNop().Sugar(),
	}
}

func cachedCtx() context.Context {
	return context.WithValue(context.Background(), ModeKey, ModeCached)
}

func noCacheCtx() context.Context {
	return context.WithValue(context.Background(), ModeKey, ModeNoCache)
}

// TestGetReturnsDeepCopy ensures a cache hit does not share mutable state with
// the stored entry, so callers can mutate the returned items (as hydration does
// in place) without corrupting the cache or racing other callers.
func TestGetReturnsDeepCopy(t *testing.T) {
	c := newTestCacher()
	ctx := cachedCtx()

	original := []testItem{{Name: "a", Tags: []string{"x"}}}
	if err := c.Store(ctx, "k", &caches.Query{Dest: &original}); err != nil {
		t.Fatalf("store: %v", err)
	}

	got := c.Get(ctx, "k")
	if got == nil {
		t.Fatal("expected a cache hit")
	}

	items := got.Dest.(*[]testItem)
	if len(*items) != 1 || (*items)[0].Name != "a" {
		t.Fatalf("unexpected cached value: %+v", items)
	}

	// Mutate the returned copy.
	(*items)[0].Name = "mutated"
	(*items)[0].Tags[0] = "mutated"

	// The stored entry must be unaffected.
	again := c.Get(ctx, "k")
	againItems := *again.Dest.(*[]testItem)
	if againItems[0].Name != "a" || againItems[0].Tags[0] != "x" {
		t.Fatalf("cache entry was mutated: %+v", againItems)
	}
}

// TestNoCacheModeInvalidates ensures that a query run with ModeNoCache (e.g. a
// user refresh) evicts any existing cached entry instead of leaving stale data
// to be served for the rest of the TTL.
func TestNoCacheModeInvalidates(t *testing.T) {
	c := newTestCacher()

	original := []testItem{{Name: "a"}}
	if err := c.Store(cachedCtx(), "k", &caches.Query{Dest: &original}); err != nil {
		t.Fatalf("store: %v", err)
	}

	if got := c.Get(cachedCtx(), "k"); got == nil {
		t.Fatal("expected a cache hit before invalidation")
	}

	if got := c.Get(noCacheCtx(), "k"); got != nil {
		t.Fatal("ModeNoCache must not return a cached value")
	}

	if got := c.Get(cachedCtx(), "k"); got != nil {
		t.Fatal("expected the entry to have been evicted by ModeNoCache")
	}
}

// TestWarmModeDoesNotServe ensures ModeWarm populates but never reads the cache.
func TestWarmModeDoesNotServe(t *testing.T) {
	c := newTestCacher()
	ctx := context.WithValue(context.Background(), ModeKey, ModeWarm)

	original := []testItem{{Name: "a"}}
	if err := c.Store(ctx, "k", &caches.Query{Dest: &original}); err != nil {
		t.Fatalf("store: %v", err)
	}

	if got := c.Get(ctx, "k"); got != nil {
		t.Fatal("ModeWarm must not serve from the cache")
	}
}
