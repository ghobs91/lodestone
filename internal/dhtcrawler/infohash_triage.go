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
	for {
		select {
		case <-ctx.Done():
			return
		case reqs := <-c.infoHashTriage.Out():
			allHashes := make([]protocol.ID, 0, len(reqs))

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

			filteredHashMap := make(map[protocol.ID]struct{}, len(filteredHashes))
			valuers := make([]driver.Valuer, 0, len(filteredHashes))

			for _, h := range filteredHashes {
				filteredHashMap[h] = struct{}{}

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

			foundTorrents := make(map[protocol.ID]triageResult)
			for _, t := range result {
				foundTorrents[t.InfoHash] = *t
			}

			for h := range filteredHashMap {
				r := reqMap[h]
				if t, ok := foundTorrents[r.infoHash]; !ok ||
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
			}
		}
	}
}

type triageResult struct {
	InfoHash    protocol.ID
	FilesStatus model.FilesStatus
	FilesCount  model.NullUint
	Seeders     model.NullUint
	Leechers    model.NullUint
	UpdatedAt   time.Time
}
