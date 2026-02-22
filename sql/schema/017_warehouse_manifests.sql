CREATE TABLE warehouse_manifests (
    id TEXT PRIMARY KEY,
    destination_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE warehouse_manifest_ship_requests (
    manifest_id TEXT NOT NULL REFERENCES warehouse_manifests(id),
    ship_request_id TEXT NOT NULL REFERENCES ship_requests(id),
    PRIMARY KEY (manifest_id, ship_request_id)
);
