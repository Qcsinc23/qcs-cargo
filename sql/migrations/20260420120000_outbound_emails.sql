-- Phase 3.2 (INC-001 part B): durable outbound email queue.
-- +goose Up
CREATE TABLE IF NOT EXISTS outbound_emails (
    id TEXT PRIMARY KEY,
    template TEXT NOT NULL,
    recipient TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    scheduled_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    sent_at TEXT,
    CHECK (status IN ('pending', 'in_progress', 'sent', 'failed')),
    CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_outbound_emails_pending
    ON outbound_emails(status, scheduled_at);

-- +goose Down
DROP INDEX IF EXISTS idx_outbound_emails_pending;
DROP TABLE IF EXISTS outbound_emails;
