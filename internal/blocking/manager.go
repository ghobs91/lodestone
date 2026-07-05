package blocking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/ghobs91/lodestone/internal/bloom"
	"github.com/ghobs91/lodestone/internal/protocol"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Manager interface {
	Filter(ctx context.Context, hashes []protocol.ID) ([]protocol.ID, error)
	Block(ctx context.Context, hashes []protocol.ID, flush bool) error
	Flush(ctx context.Context) error
}

type manager struct {
	mu            sync.Mutex
	pool          *pgxpool.Pool
	buffer        map[protocol.ID]struct{}
	filter        *bloom.StableBloomFilter
	filterLoaded  bool
	maxBufferSize int
	lastFlushedAt time.Time
	maxFlushWait  time.Duration
}

func (m *manager) Filter(ctx context.Context, hashes []protocol.ID) ([]protocol.ID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Lazy-load the bloom filter from the database on first use.
	if !m.filterLoaded {
		if loadErr := m.loadFilter(ctx); loadErr != nil {
			return nil, loadErr
		}
	}

	filtered := make([]protocol.ID, 0, len(hashes))

	for _, hash := range hashes {
		if _, ok := m.buffer[hash]; ok {
			continue
		}

		if m.filter != nil && m.filter.Test(hash[:]) {
			continue
		}

		filtered = append(filtered, hash)
	}

	return filtered, nil
}

func (m *manager) Block(ctx context.Context, hashes []protocol.ID, flush bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Lazy-load on first use.
	if !m.filterLoaded {
		if loadErr := m.loadFilter(ctx); loadErr != nil {
			return loadErr
		}
	}

	for _, hash := range hashes {
		m.buffer[hash] = struct{}{}
	}

	if flush || m.shouldFlush() {
		if flushErr := m.persist(ctx); flushErr != nil {
			return flushErr
		}
	}

	return nil
}

func (m *manager) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.buffer) == 0 {
		return nil
	}

	return m.persist(ctx)
}

const blockedTorrentsBloomFilterKey = "blocked_torrents"

// loadFilter reads the bloom filter from PostgreSQL Large Objects on first use.
// This is separated from persist so that Filter (read-only) doesn't trigger I/O.
func (m *manager) loadFilter(ctx context.Context) error {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction for bloom filter load: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	bf := bloom.NewDefaultStableBloomFilter()

	var nullOid sql.NullInt32
	err = tx.QueryRow(ctx, "SELECT oid FROM bloom_filters WHERE key = $1", blockedTorrentsBloomFilterKey).
		Scan(&nullOid)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to query bloom filter oid: %w", err)
	}

	if nullOid.Valid {
		lobs := tx.LargeObjects()
		obj, openErr := lobs.Open(ctx, uint32(nullOid.Int32), pgx.LargeObjectModeRead)
		if openErr != nil {
			return fmt.Errorf("failed to open large object for reading: %w", openErr)
		}

		_, readErr := bf.ReadFrom(obj)
		obj.Close()

		if readErr != nil {
			return fmt.Errorf("failed to read bloom filter: %w", readErr)
		}
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("failed to commit bloom filter load: %w", commitErr)
	}

	m.filter = bf
	m.filterLoaded = true
	return nil
}

// persist writes any buffered blocked hashes to the database and updates the
// persisted bloom filter. This is only called from Block (when the buffer is
// full) or from Flush (explicit/manual).
func (m *manager) persist(ctx context.Context) error {
	hashes := slices.Collect(maps.Keys(m.buffer))
	if len(hashes) == 0 && m.filter != nil {
		// Nothing to persist and filter already loaded — no-op.
		return nil
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(hashes) > 0 {
		_, err = tx.Exec(ctx, "DELETE FROM torrents WHERE info_hash = any($1)", hashes)
		if err != nil {
			return fmt.Errorf("failed to delete from torrents table: %w", err)
		}
	}

	// Use the in-memory filter if available (it already contains previously-
	// blocked hashes), otherwise start fresh. Add any buffered hashes.
	var bf *bloom.StableBloomFilter
	if m.filter != nil {
		bf = m.filter
	} else {
		bf = bloom.NewDefaultStableBloomFilter()
	}
	for _, hash := range hashes {
		bf.Add(hash[:])
	}

	lobs := tx.LargeObjects()

	found := false
	var oid uint32
	var nullOid sql.NullInt32

	err = tx.QueryRow(ctx, "SELECT oid FROM bloom_filters WHERE key = $1", blockedTorrentsBloomFilterKey).
		Scan(&nullOid)
	if err == nil {
		found = true
		if nullOid.Valid {
			oid = uint32(nullOid.Int32)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to get bloom filter object ID: %w", err)
	}

	if oid == 0 {
		oid, err = lobs.Create(ctx, 0)
		if err != nil {
			return fmt.Errorf("failed to create large object: %w", err)
		}
	}

	obj, err := lobs.Open(ctx, oid, pgx.LargeObjectModeWrite)
	if err != nil {
		return fmt.Errorf("failed to open large object for writing: %w", err)
	}

	_, err = bf.WriteTo(obj)
	if err != nil {
		return fmt.Errorf("failed to write to large object: %w", err)
	}
	obj.Close()

	now := time.Now()
	if !found {
		_, err = tx.Exec(ctx,
			"INSERT INTO bloom_filters (key, oid, created_at, updated_at) VALUES ($1, $2, $3, $4)",
			blockedTorrentsBloomFilterKey, oid, now, now)
		if err != nil {
			return fmt.Errorf("failed to save new bloom filter record: %w", err)
		}
	} else if !nullOid.Valid {
		_, err = tx.Exec(ctx,
			"UPDATE bloom_filters SET oid = $1, updated_at = $2 WHERE key = $3",
			oid, now, blockedTorrentsBloomFilterKey)
		if err != nil {
			return fmt.Errorf("failed to update bloom filter record: %w", err)
		}
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr)
	}

	m.buffer = make(map[protocol.ID]struct{})
	m.filter = bf
	m.lastFlushedAt = now

	return nil
}

func (m *manager) shouldFlush() bool {
	return len(m.buffer) >= m.maxBufferSize || time.Since(m.lastFlushedAt) >= m.maxFlushWait
}
