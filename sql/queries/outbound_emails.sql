-- name: EnqueueOutboundEmail :exec
-- Phase 3.2 (INC-001b): insert a row representing a transactional email
-- that needs to be sent. The worker drains rows where status='pending'
-- AND scheduled_at <= now.
INSERT INTO outbound_emails (
    id, template, recipient, payload_json, status,
    attempt_count, scheduled_at, created_at
) VALUES (
    ?, ?, ?, ?, 'pending', 0, ?, ?
);

-- name: ClaimPendingOutboundEmails :many
-- Atomically mark up to N pending rows as 'in_progress' so the worker
-- can drain them without contending with a parallel run. Returns the
-- claimed rows. Single-replica deployment makes the simple two-step
-- (UPDATE + SELECT) pattern safe inside one transaction.
SELECT id, template, recipient, payload_json, attempt_count
FROM outbound_emails
WHERE status = 'pending' AND scheduled_at <= ?
ORDER BY scheduled_at
LIMIT ?;

-- name: MarkOutboundEmailInProgress :exec
UPDATE outbound_emails
SET status = 'in_progress'
WHERE id = ? AND status = 'pending';

-- name: MarkOutboundEmailSent :exec
UPDATE outbound_emails
SET status = 'sent', sent_at = ?, last_error = NULL
WHERE id = ?;

-- name: MarkOutboundEmailFailed :exec
UPDATE outbound_emails
SET status = ?, attempt_count = attempt_count + 1,
    last_error = ?, scheduled_at = ?
WHERE id = ?;

-- name: CountOutboundEmailsByStatus :one
SELECT COUNT(*) FROM outbound_emails WHERE status = ?;

-- name: ReapStuckOutboundEmails :execrows
-- Pass 2.5 HIGH-10 fix: rows that the worker marked 'in_progress' but
-- never finished (panic, host crash, kill mid-send) are stuck forever
-- since ClaimPendingOutboundEmails only sees 'pending'. Reset stale
-- in_progress rows back to pending and increment attempt_count so the
-- existing maxOutboundAttempts budget still bounds total retries.
UPDATE outbound_emails
SET status = 'pending',
    attempt_count = attempt_count + 1,
    scheduled_at = ?,
    last_error = COALESCE(last_error, '') || ';reaped'
WHERE status = 'in_progress' AND scheduled_at < ?;
