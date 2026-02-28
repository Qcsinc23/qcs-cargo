-- Sent notifications dedupe ledger.
-- +goose Up
CREATE TABLE IF NOT EXISTS sent_notifications (
    id TEXT PRIMARY KEY,
    notification_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    recipient_email TEXT NOT NULL,
    sent_at TEXT NOT NULL,
    UNIQUE(notification_type, resource_id, recipient_email)
);

-- +goose Down
DROP TABLE IF EXISTS sent_notifications;
