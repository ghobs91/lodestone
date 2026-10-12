-- +goose Up
-- +goose StatementBegin

-- Job status 'processing' is set while a worker is executing a job, so the job
-- row lock/transaction can be released between claim and completion.
ALTER TYPE queue_job_status ADD VALUE IF NOT EXISTS 'processing';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- PostgreSQL does not support removing an enum value, so this is a no-op.
-- (Recreating the type would require rewriting the queue_jobs table.)

-- +goose StatementEnd
