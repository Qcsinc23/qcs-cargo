-- Pass 2.5 MED-16 fix: push subscription endpoints must be globally
-- unique, not unique only per (user_id, endpoint). Under the original
-- (user_id, endpoint) UNIQUE constraint, an attacker who learned a
-- victim's endpoint URL (e.g. via a leaked log or a compromised
-- service worker) could re-register that same endpoint under their
-- own user_id and silently misroute server-sent pushes.
--
-- We rebuild push_subscriptions with UNIQUE(endpoint). The original
-- schema (sql/migrations/20260301080000_notifications_platform_wave11.sql)
-- declared p256dh and auth as NOT NULL; preserve that here so the
-- new table is structurally identical aside from the constraint
-- change.
--
-- If duplicate endpoints already exist (a sign of pre-existing abuse
-- or just a benign multi-account device), keep the row with the most
-- recent created_at and discard the rest.

-- +goose Up
DROP TABLE IF EXISTS push_subscriptions__new;
CREATE TABLE push_subscriptions__new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO push_subscriptions__new (id, user_id, endpoint, p256dh, auth, created_at, updated_at)
SELECT id, user_id, endpoint, p256dh, auth, created_at, updated_at
FROM push_subscriptions ps
WHERE ps.created_at = (
    SELECT MAX(created_at) FROM push_subscriptions WHERE endpoint = ps.endpoint
)
AND ps.id = (
    SELECT id FROM push_subscriptions
    WHERE endpoint = ps.endpoint AND created_at = ps.created_at
    ORDER BY id LIMIT 1
);

DROP TABLE push_subscriptions;
ALTER TABLE push_subscriptions__new RENAME TO push_subscriptions;

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user
    ON push_subscriptions(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_push_subscriptions_user;
DROP TABLE IF EXISTS push_subscriptions__old;
CREATE TABLE push_subscriptions__old (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    endpoint TEXT NOT NULL,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(user_id, endpoint)
);
INSERT INTO push_subscriptions__old (id, user_id, endpoint, p256dh, auth, created_at, updated_at)
SELECT id, user_id, endpoint, p256dh, auth, created_at, updated_at FROM push_subscriptions;
DROP TABLE push_subscriptions;
ALTER TABLE push_subscriptions__old RENAME TO push_subscriptions;
