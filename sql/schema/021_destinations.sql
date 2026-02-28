CREATE TABLE destinations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    code TEXT NOT NULL,
    capital TEXT NOT NULL,
    usd_per_lb REAL NOT NULL,
    transit_days_min INTEGER NOT NULL,
    transit_days_max INTEGER NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 100,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
