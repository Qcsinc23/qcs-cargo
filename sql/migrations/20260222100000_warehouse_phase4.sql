-- Warehouse Phase 4: ship_requests warehouse fields, locker_packages.booking_id, warehouse_bays, manifests.
-- +goose Up
ALTER TABLE ship_requests ADD COLUMN consolidated_weight_lbs REAL;
ALTER TABLE ship_requests ADD COLUMN staging_bay TEXT;
ALTER TABLE ship_requests ADD COLUMN manifest_id TEXT;

ALTER TABLE locker_packages ADD COLUMN booking_id TEXT REFERENCES bookings(id);

CREATE TABLE IF NOT EXISTS warehouse_bays (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    zone TEXT,
    destination_id TEXT,
    capacity INTEGER NOT NULL DEFAULT 0,
    current_count INTEGER NOT NULL DEFAULT 0
);

-- Warehouse manifests (PRD 6.10)
CREATE TABLE IF NOT EXISTS warehouse_manifests (
    id TEXT PRIMARY KEY,
    destination_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS warehouse_manifest_ship_requests (
    manifest_id TEXT NOT NULL REFERENCES warehouse_manifests(id),
    ship_request_id TEXT NOT NULL REFERENCES ship_requests(id),
    PRIMARY KEY (manifest_id, ship_request_id)
);

-- +goose Down
-- Rebuild altered tables to remove Phase 4 columns for broad SQLite compatibility.
PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS warehouse_manifest_ship_requests;
DROP TABLE IF EXISTS warehouse_manifests;
DROP TABLE IF EXISTS warehouse_bays;

DROP TABLE IF EXISTS ship_requests__new_202602221000;
CREATE TABLE ship_requests__new_202602221000 (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    confirmation_code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    destination_id TEXT NOT NULL,
    recipient_id TEXT,
    service_type TEXT NOT NULL,
    consolidate INTEGER NOT NULL DEFAULT 1,
    special_instructions TEXT,
    subtotal REAL NOT NULL DEFAULT 0,
    service_fees REAL NOT NULL DEFAULT 0,
    insurance REAL NOT NULL DEFAULT 0,
    discount REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    payment_status TEXT,
    stripe_payment_intent_id TEXT,
    customs_status TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO ship_requests__new_202602221000 (
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    created_at, updated_at
)
SELECT
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    created_at, updated_at
FROM ship_requests;

DROP TABLE ship_requests;
ALTER TABLE ship_requests__new_202602221000 RENAME TO ship_requests;

CREATE INDEX IF NOT EXISTS idx_ship_requests_user_status ON ship_requests(user_id, status);
CREATE INDEX IF NOT EXISTS idx_ship_requests_confirmation ON ship_requests(confirmation_code);

DROP TABLE IF EXISTS locker_packages__new_202602221000;
CREATE TABLE locker_packages__new_202602221000 (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    suite_code TEXT NOT NULL,
    tracking_inbound TEXT,
    carrier_inbound TEXT,
    sender_name TEXT,
    sender_address TEXT,
    weight_lbs REAL,
    length_in REAL,
    width_in REAL,
    height_in REAL,
    arrival_photo_url TEXT,
    condition TEXT,
    storage_bay TEXT,
    status TEXT NOT NULL DEFAULT 'stored',
    arrived_at TEXT,
    free_storage_expires_at TEXT,
    disposed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO locker_packages__new_202602221000 (
    id, user_id, suite_code, tracking_inbound, carrier_inbound,
    sender_name, sender_address, weight_lbs, length_in, width_in, height_in,
    arrival_photo_url, condition, storage_bay, status,
    arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
)
SELECT
    id, user_id, suite_code, tracking_inbound, carrier_inbound,
    sender_name, sender_address, weight_lbs, length_in, width_in, height_in,
    arrival_photo_url, condition, storage_bay, status,
    arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages;

DROP TABLE locker_packages;
ALTER TABLE locker_packages__new_202602221000 RENAME TO locker_packages;

CREATE INDEX IF NOT EXISTS idx_locker_packages_user_status ON locker_packages(user_id, status);
CREATE INDEX IF NOT EXISTS idx_locker_packages_suite_code ON locker_packages(suite_code);
CREATE INDEX IF NOT EXISTS idx_locker_packages_arrived_at ON locker_packages(arrived_at);
CREATE INDEX IF NOT EXISTS idx_locker_packages_free_storage ON locker_packages(free_storage_expires_at, status);

PRAGMA foreign_keys = ON;
