CREATE TABLE warehouse_bays (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    zone TEXT,
    destination_id TEXT,
    capacity INTEGER NOT NULL DEFAULT 0,
    current_count INTEGER NOT NULL DEFAULT 0
);
