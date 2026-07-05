package dhtcrawler

import (
	"context"
	"database/sql/driver"
	"time"

	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/protocol"
)

// runInfoHashTriage receives discovered hashes on the infoHashTriage channel, determines if they should be crawled,
// and forwards them to the appropriate channel. Possible outcomes are:
//  1. The hash is not in the database, so it is forwarded to the getPeers channel to attempt
//     retrieval of the meta info.
//  2. The hash is in the database, but we don't have the full details of the torrent (for example it was imported
//     outside the DHT crawler, and so we don't have the files info), so it is forwarded to the getPeers channel to
//     attempt retrieval of the meta info.
//  3. The hash is in the database, but the seeders/leechers are not known or are outdated,
//     so it is forwarded to the scrape channel.
//  4. The hash is in the database and the seeders/leechers are known and up to date, so it is discarded.
func (c *crawler) runInfoHashTriage(ctx context.Context) {
	// Local cache of recently-triaged hashes to avoid redundant DB lookups.
	// Hashes that were already in the DB (and up-to-date) are cached briefly.
	triageCache := newTriageCache(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case reqs := <-c.infoHashTriage.Out():
			allHashes := make([]protocol.ID, 0,
				len(reqs))

			reqMap := make(map[protocol.ID]nodeHasPeersForHash, len(reqs))
			for _, r := range reqs {
				if _, ok := reqMap[r.infoHash]; ok {
					continue
				}

				allHashes = append(allHashes, r.infoHash)
				reqMap[r.infoHash] = r
			}

			filteredHashes, filterErr := c.blockingManager.Filter(ctx, allHashes)
			if filterErr != nil {
				c.logger.Errorf("failed to filter infohashes: %s", filterErr.Error())
				break
			}

			if len(filteredHashes) == 0 {
				break
			}

			// Separate hashes we can resolve from the in-memory cache from
			// those that still need a DB lookup.
			dbLookupHashes := make([]protocol.ID, 0, len(filteredHashes))
			cachedResults := make(map[protocol.ID]triageResult, len(filteredHashes))

			for _, h := range filteredHashes {
				if tr, ok := triageCache.get(h); ok {
					cachedResults[h] = tr
				} else {
					dbLookupHashes = append(dbLookupHashes, h)
				}
			}

			// Only query the DB for hashes not in the cache.
			if len(dbLookupHashes) > 0 {
				valuers := make([]driver.Valuer, 0, len(dbLookupHashes))
				for _, h := range dbLookupHashes {
					valuers = append(valuers, h)
				}

				var result []*triageResult
				// Wrap the lookup in a transaction to get a consistent snapshot,
				// avoiding races with the persist pipeline committing new torrents.
				if txErr := c.dao.Transaction(func(tx *dao.Query) error {
					return tx.Torrent.WithContext(ctx).Select(
						tx.Torrent.InfoHash,
						tx.Torrent.FilesStatus,
						tx.Torrent.FilesCount,
						tx.TorrentsTorrentSource.Seeders,
						tx.TorrentsTorrentSource.Leechers,
						tx.TorrentsTorrentSource.UpdatedAt,
					).LeftJoin(
						tx.TorrentsTorrentSource,
						tx.Torrent.InfoHash.EqCol(tx.TorrentsTorrentSource.InfoHash),
						tx.TorrentsTorrentSource.Source.Eq("dht"),
					).Where(
						tx.Torrent.InfoHash.In(valuers...),
					).UnderlyingDB().Find(&result).Error
				}); txErr != nil {
					c.logger.Errorf("failed to search existing torrents: %s", txErr.Error())
					break
				}

				for _, t := range result {
					cachedResults[t.InfoHash] = *t
					// Cache results for hashes that are already complete and
					// up-to-date (outcome #4). Others are transient and should
					// not be cached.
					if t.FilesStatus != model.FilesStatusNoInfo &&
						(t.FilesStatus == model.FilesStatusSingle || t.FilesCount.Valid) &&
						!(t.FilesStatus == model.FilesStatusOverThreshold && t.FilesCount.Uint <= c.saveFilesThreshold) &&
						t.Seeders.Valid && t.Leechers.Valid &&
						!t.UpdatedAt.Before(time.Now().Add(-c.rescrapeThreshold)) {
						triageCache.set(t.InfoHash, *t)
					}
				}
			}

			// Route each hash based on its triage result.
			for h := range filteredHashes {
				r := reqMap[h]
				t, ok := cachedResults[h]
				if !ok ||
					t.FilesStatus == model.FilesStatusNoInfo ||
					(t.FilesStatus != model.FilesStatusSingle && !t.FilesCount.Valid) ||
					(t.FilesStatus == model.FilesStatusOverThreshold && t.FilesCount.Uint <= c.saveFilesThreshold) {
					select {
					case <-ctx.Done():
						return
					case c.getPeers.In() <- r:
						continue
					}
				} else if (!t.Seeders.Valid || !t.Leechers.Valid) ||
					t.UpdatedAt.Before(time.Now().Add(-c.rescrapeThreshold)) {
					select {
					case <-ctx.Done():
						return
					case c.scrape.In() <- r:
						continue
					}
				}

				// Update the cached entry if we're going to scrape (the
				// updated_at will change soon, so remove from cache).
				triageCache.remove(h)
			}
		}
	}
}

// triageCache is a small, time-bounded cache for triage results that avoids
// redundant DB lookups for hashes we already know about.
type triageCache struct {
	entries map[protocol.ID]triageCacheEntry
	ttl     time.Duration
}

type triageCacheEntry struct {
	result    triageResult
	expiresAt time.Time
}

func newTriageCache(ttl time.Duration) *triageCache {
	return &triageCache{
		entries: make(map[protocol.ID]triageCacheEntry),
		ttl:     ttl,
	}
}

func (tc *triageCache) get(hash protocol.ID) (triageResult, bool) {
	e, ok := tc.entries[hash]
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			delete(tc.entries, hash)
		}
		return triageResult{}, false
	}
	return e.result, true
}

func (tc *triageCache) set(hash protocol.ID, result triageResult) {
	// Limit cache size to prevent unbounded growth.
	if len(tc.entries) >= 100_000 {
		// Evict ~10% of entries (simple first-key eviction).
		i := 0
		for k := range tc.entries {
			delete(tc.entries, k)
			i++
			if i >= 10_000 {
				break
			}
		}
	}
	tc.entries[hash] = triageCacheEntry{
		result:    result,
		expiresAt: time.Now().Add(tc.ttl),
	}
}

func (tc *triageCache) remove(hash protocol.ID) {
	delete(tc.entries, hash)
}

type triageResult struct {
	InfoHash    protocol.ID
	FilesStatus model.FilesStatus
	FilesCount  model.NullUint
	Seeders     model.NullUint
	Leechers    model.NullUint
	UpdatedAt   time.Time
}
