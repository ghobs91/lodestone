-- +goose Up
-- +goose StatementBegin

-- Unknown classifications are no longer persisted: the processor deletes any
-- torrent whose classified content type is null. Purge the existing rows that
-- only have unknown (null content_type) torrent_contents.
--
-- Torrents with no torrent_contents at all are left alone: they have not been
-- classified yet and will be deleted by the processor if classification fails
-- to determine a content type.
delete from torrents t
where exists (
  select 1 from torrent_contents tc
  where tc.info_hash = t.info_hash
)
and not exists (
  select 1 from torrent_contents tc
  where tc.info_hash = t.info_hash
    and tc.content_type is not null
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Deleted torrents cannot be restored.

-- +goose StatementEnd
