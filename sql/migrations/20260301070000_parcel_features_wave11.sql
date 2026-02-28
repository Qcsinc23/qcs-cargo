-- +goose Up
CREATE TABLE IF NOT EXISTS parcel_consolidation_previews (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    package_ids_json TEXT NOT NULL,
    package_count INTEGER NOT NULL DEFAULT 0,
    total_weight_lbs REAL NOT NULL DEFAULT 0,
    pre_consolidation_billable_lbs REAL NOT NULL DEFAULT 0,
    post_consolidation_billable_lbs REAL NOT NULL DEFAULT 0,
    estimated_savings_lbs REAL NOT NULL DEFAULT 0,
    destination_id TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_parcel_consolidation_previews_user_created
    ON parcel_consolidation_previews(user_id, created_at);

CREATE TABLE IF NOT EXISTS assisted_purchase_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    recipient_id TEXT REFERENCES recipients(id),
    store_url TEXT NOT NULL,
    item_name TEXT NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    estimated_cost_usd REAL NOT NULL DEFAULT 0,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_assisted_purchase_requests_user_created
    ON assisted_purchase_requests(user_id, created_at);

CREATE TABLE IF NOT EXISTS customs_preclearance_docs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    ship_request_id TEXT REFERENCES ship_requests(id),
    locker_package_id TEXT REFERENCES locker_packages(id),
    doc_type TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_url TEXT NOT NULL,
    mime_type TEXT,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'uploaded',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_customs_preclearance_docs_user_created
    ON customs_preclearance_docs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_customs_preclearance_docs_ship_request
    ON customs_preclearance_docs(ship_request_id, created_at);

CREATE TABLE IF NOT EXISTS delivery_signatures (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    ship_request_id TEXT NOT NULL REFERENCES ship_requests(id),
    signer_name TEXT NOT NULL,
    signature_data TEXT NOT NULL,
    captured_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(user_id, ship_request_id)
);
CREATE INDEX IF NOT EXISTS idx_delivery_signatures_user_ship_request
    ON delivery_signatures(user_id, ship_request_id);

CREATE TABLE IF NOT EXISTS loyalty_ledger (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    points_delta INTEGER NOT NULL,
    reason TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_loyalty_ledger_user_created
    ON loyalty_ledger(user_id, created_at);

CREATE TABLE IF NOT EXISTS data_import_jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    import_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'completed',
    payload_preview TEXT,
    total_rows INTEGER NOT NULL DEFAULT 0,
    imported_rows INTEGER NOT NULL DEFAULT 0,
    failed_rows INTEGER NOT NULL DEFAULT 0,
    error_summary TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_data_import_jobs_user_created
    ON data_import_jobs(user_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS data_import_jobs;
DROP TABLE IF EXISTS loyalty_ledger;
DROP TABLE IF EXISTS delivery_signatures;
DROP TABLE IF EXISTS customs_preclearance_docs;
DROP TABLE IF EXISTS assisted_purchase_requests;
DROP TABLE IF EXISTS parcel_consolidation_previews;
