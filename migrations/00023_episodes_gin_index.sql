-- +goose Up
-- +goose StatementBegin

-- The only index on torrent_contents.episodes was a plain btree, which cannot
-- serve the JSONB containment (@>) predicates used by episode searches. Replace
-- it with a GIN index that can.
drop index if exists torrent_contents_episodes_idx;
create index if not exists torrent_contents_episodes_gin_idx
  on torrent_contents using gin (episodes jsonb_path_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

drop index if exists torrent_contents_episodes_gin_idx;
create index if not exists torrent_contents_episodes_idx on torrent_contents (episodes);

-- +goose StatementEnd
