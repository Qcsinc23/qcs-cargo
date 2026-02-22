CREATE TABLE templates (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    service_type TEXT NOT NULL,
    destination_id TEXT NOT NULL,
    recipient_id TEXT,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
