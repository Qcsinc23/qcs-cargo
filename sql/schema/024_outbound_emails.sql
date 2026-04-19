-- Phase 3.2 (INC-001 part B): durable outbound email queue.
--
-- Replaces the previous "send inline, log on failure" pattern with a row
-- per outbound email. The send loop in internal/jobs/outbound_email.go
-- drains pending rows with bounded retry, so a transient Resend outage
-- no longer silently drops customer notifications. Permanent failures
-- (after the attempt budget) are marked status='failed' for ops review.
CREATE TABLE outbound_emails (
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

CREATE INDEX idx_outbound_emails_pending
    ON outbound_emails(status, scheduled_at);
