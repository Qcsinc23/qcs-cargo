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
-- SQLite: cannot easily drop columns; leave ship_requests/locker_packages columns or document manual step.
DROP TABLE IF EXISTS warehouse_manifest_ship_requests;
DROP TABLE IF EXISTS warehouse_manifests;
DROP TABLE IF EXISTS warehouse_bays;
