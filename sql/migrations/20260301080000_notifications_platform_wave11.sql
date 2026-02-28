-- +goose Up
CREATE TABLE IF NOT EXISTS in_app_notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    level TEXT NOT NULL DEFAULT 'info',
    link_url TEXT,
    read_at TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_in_app_notifications_user_created
    ON in_app_notifications(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    endpoint TEXT NOT NULL,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(user_id, endpoint)
);

CREATE TABLE IF NOT EXISTS moderation_items (
    id TEXT PRIMARY KEY,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_moderation_items_status_created
    ON moderation_items(status, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_moderation_items_status_created;
DROP TABLE IF EXISTS moderation_items;
DROP TABLE IF EXISTS push_subscriptions;
DROP INDEX IF EXISTS idx_in_app_notifications_user_created;
DROP TABLE IF EXISTS in_app_notifications;
