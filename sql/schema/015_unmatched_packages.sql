-- Unmatched packages (warehouse receive without valid suite code). PRD 2.12.
CREATE TABLE unmatched_packages (
    id TEXT PRIMARY KEY,
    carrier TEXT,
    tracking_number TEXT,
    label_text TEXT,
    photo_url TEXT,
    weight_lbs REAL,
    status TEXT NOT NULL DEFAULT 'pending',
    matched_user_id TEXT,
    resolution_notes TEXT,
    received_at TEXT NOT NULL,
    resolved_at TEXT,
    created_at TEXT NOT NULL
);
