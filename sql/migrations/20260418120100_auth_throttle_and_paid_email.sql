-- Audit Pass 2 follow-ups (M-4 throttle table, L-4 payment-email idempotency).
-- +goose Up
PRAGMA foreign_keys = OFF;

-- M-4: per-account auth attempt window, used to throttle magic-link
-- requests, password-reset requests, and verification resends. Distinct
-- from the in-memory IP lockout (middleware/rate_limit.go) so that a
-- restart does not wipe per-account counters.
CREATE TABLE IF NOT EXISTS auth_request_log (
    id TEXT PRIMARY KEY,
    bucket TEXT NOT NULL,         -- e.g. "magic_link:email@x"
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_request_log_bucket_time
    ON auth_request_log (bucket, created_at);

-- L-4: idempotency marker so a Stripe webhook retry of payment_intent.succeeded
-- does not re-send the "Paid" email when the ship request was already paid.
ALTER TABLE ship_requests ADD COLUMN paid_email_sent_at TEXT;

PRAGMA foreign_keys = ON;

-- +goose Down
DROP INDEX IF EXISTS idx_auth_request_log_bucket_time;
DROP TABLE IF EXISTS auth_request_log;
