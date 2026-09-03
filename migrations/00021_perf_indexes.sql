-- +goose Up
-- +goose StatementBegin

-- Batch reclassify scan: WHERE info_hash > ? AND updated_at < ? ORDER BY info_hash LIMIT ?
create index if not exists torrents_info_hash_updated_at_idx on torrents (info_hash, updated_at);
create index if not exists torrents_updated_at_idx on torrents (updated_at);

-- Queue poll: WHERE queue = ? AND status IN (...) AND run_after <= ? ORDER BY run_after
create index if not exists queue_jobs_queue_status_run_after_idx on queue_jobs (queue, status, run_after);

-- Torrent-content lookup by hash (TorrentContentInfoHashCriteria)
create index if not exists torrent_contents_info_hash_idx on torrent_contents (info_hash);

-- Content search by type ordered by recency
create index if not exists content_type_release_date_idx on content (type, release_date);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop index if exists torrents_info_hash_updated_at_idx;
drop index if exists torrents_updated_at_idx;
drop index if exists queue_jobs_queue_status_run_after_idx;
drop index if exists torrent_contents_info_hash_idx;
drop index if exists content_type_release_date_idx;

-- +goose StatementEnd
