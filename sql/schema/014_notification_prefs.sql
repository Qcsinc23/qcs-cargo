CREATE TABLE notification_prefs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    email_enabled INTEGER NOT NULL DEFAULT 1,
    sms_enabled INTEGER NOT NULL DEFAULT 0,
    push_enabled INTEGER NOT NULL DEFAULT 1,
    on_package_arrived INTEGER NOT NULL DEFAULT 1,
    on_storage_expiry INTEGER NOT NULL DEFAULT 1,
    on_ship_updates INTEGER NOT NULL DEFAULT 1,
    on_inbound_updates INTEGER NOT NULL DEFAULT 1,
    daily_digest TEXT NOT NULL DEFAULT 'off',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
