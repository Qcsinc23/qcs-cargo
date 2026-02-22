CREATE TABLE inbound_tracking (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    carrier TEXT NOT NULL,
    tracking_number TEXT NOT NULL,
    retailer_name TEXT,
    expected_items TEXT,
    status TEXT NOT NULL DEFAULT 'tracking',
    locker_package_id TEXT,
    last_checked_at TEXT,
    created_at TEXT NOT NULL
);
