CREATE TABLE IF NOT EXISTS sent_notifications (
    id TEXT PRIMARY KEY,
    notification_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    recipient_email TEXT NOT NULL,
    sent_at TEXT NOT NULL,
    UNIQUE(notification_type, resource_id, recipient_email)
);
