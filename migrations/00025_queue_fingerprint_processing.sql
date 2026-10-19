-- +goose Up
-- +goose StatementBegin

-- The unique fingerprint index only covered pending/retry, so a job could be
-- claimed (status 'processing', outside the index) and a duplicate of the same
-- payload enqueued behind it. When the claimed job later failed and was set back
-- to 'retry', the update violated the index and the job was stuck in
-- 'processing' forever.
--
-- Include 'processing' in the index so a job cannot be duplicated while it is
-- queued OR in flight. Enqueues already use ON CONFLICT DO NOTHING.
--
-- Deduplicate existing rows first (prefer a runnable pending/retry job over a
-- stuck processing one, then the oldest), otherwise the index cannot be created.
DELETE FROM queue_jobs
WHERE id IN (
  SELECT id FROM (
    SELECT id, row_number() OVER (
      PARTITION BY fingerprint
      ORDER BY (status IN ('pending', 'retry')) DESC, created_at ASC, id ASC
    ) AS rn
    FROM queue_jobs
    WHERE status IN ('pending', 'retry', 'processing')
  ) ranked
  WHERE ranked.rn > 1
);

DROP INDEX IF EXISTS queue_jobs_fingerprint_idx;

CREATE UNIQUE INDEX queue_jobs_fingerprint_idx
  ON queue_jobs (fingerprint)
  WHERE status IN ('pending', 'retry', 'processing');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS queue_jobs_fingerprint_idx;

CREATE UNIQUE INDEX queue_jobs_fingerprint_idx
  ON queue_jobs (fingerprint)
  WHERE status IN ('pending', 'retry');

-- +goose StatementEnd
