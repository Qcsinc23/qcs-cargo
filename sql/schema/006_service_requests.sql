CREATE TABLE service_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    locker_package_id TEXT NOT NULL REFERENCES locker_packages(id),
    service_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    notes TEXT,
    completed_by TEXT,
    price REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    completed_at TEXT
);
