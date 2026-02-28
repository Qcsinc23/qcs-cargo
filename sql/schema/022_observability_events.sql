CREATE TABLE observability_events (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL CHECK (category IN ('error', 'analytics', 'performance', 'business')),
    event_name TEXT NOT NULL,
    user_id TEXT,
    request_id TEXT,
    path TEXT,
    method TEXT,
    status_code INTEGER,
    duration_ms REAL,
    value REAL,
    metadata_json TEXT,
    created_at TEXT NOT NULL
);
