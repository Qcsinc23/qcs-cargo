CREATE TABLE recipients (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    phone TEXT,
    destination_id TEXT NOT NULL,
    street TEXT NOT NULL,
    apt TEXT,
    city TEXT NOT NULL,
    delivery_instructions TEXT,
    is_default INTEGER NOT NULL DEFAULT 0,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
