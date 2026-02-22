CREATE TABLE shipments (
    id TEXT PRIMARY KEY,
    destination_id TEXT NOT NULL,
    manifest_id TEXT,
    ship_request_id TEXT REFERENCES ship_requests(id),
    tracking_number TEXT,
    status TEXT NOT NULL,
    total_weight REAL,
    package_count INTEGER,
    carrier TEXT,
    estimated_delivery TEXT,
    actual_delivery TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
