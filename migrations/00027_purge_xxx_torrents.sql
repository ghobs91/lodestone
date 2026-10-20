-- +goose Up
-- +goose StatementBegin

-- XXX content is never stored anymore: the processor deletes any torrent whose
-- classified content type is xxx. Purge the existing XXX torrents.
delete from torrents t
where exists (
  select 1 from torrent_contents tc
  where tc.info_hash = t.info_hash
    and tc.content_type = 'xxx'
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Deleted torrents cannot be restored.

-- +goose StatementEnd
