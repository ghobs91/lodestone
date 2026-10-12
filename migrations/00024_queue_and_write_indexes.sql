-- +goose Up
-- +goose StatementBegin

-- Queue dashboard listing sorts by ran_at (default) or created_at, with no
-- supporting index. Without these, every page load does a seq scan + sort.
create index if not exists queue_jobs_ran_at_idx on queue_jobs (ran_at desc);
create index if not exists queue_jobs_created_at_idx on queue_jobs (created_at desc);

-- Support the garbage-collection predicate, which filters processed/failed
-- jobs by ran_at. A partial index keeps it small and directly usable.
create index if not exists queue_jobs_gc_idx
  on queue_jobs (ran_at)
  where status in ('processed', 'failed');

-- Remove redundant indexes that only add write amplification on hot tables:
--   * (queue, status) is a prefix of (queue, status, run_after)
--   * (id, queue, status, priority, run_after) is unusable because id is the PK
--   * (seeders)/(leechers) are never filtered on, only selected/updated
--   * (adult)/(popularity)/(original_language) are never filtered on
--   * torrent_contents (content_source) is a prefix of (content_source, content_id)
drop index if exists queue_jobs_queue_status_idx;
drop index if exists queue_jobs_id_queue_status_priority_run_after_idx;
drop index if exists torrents_torrent_sources_seeders_idx;
drop index if exists torrents_torrent_sources_leechers_idx;
drop index if exists content_adult_idx;
drop index if exists content_popularity_idx;
drop index if exists content_original_language_idx;
drop index if exists torrent_contents_content_source_idx;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop index if exists queue_jobs_ran_at_idx;
drop index if exists queue_jobs_created_at_idx;
drop index if exists queue_jobs_gc_idx;

create index if not exists queue_jobs_queue_status_idx on queue_jobs (queue, status);
create index if not exists queue_jobs_id_queue_status_priority_run_after_idx on queue_jobs (id, queue, status, priority, run_after);
create index if not exists torrents_torrent_sources_seeders_idx on torrents_torrent_sources (seeders);
create index if not exists torrents_torrent_sources_leechers_idx on torrents_torrent_sources (leechers);
create index if not exists content_adult_idx on content (adult);
create index if not exists content_popularity_idx on content (popularity);
create index if not exists content_original_language_idx on content (original_language);
create index if not exists torrent_contents_content_source_idx on torrent_contents (content_source);

-- +goose StatementEnd
