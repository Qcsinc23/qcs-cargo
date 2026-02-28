-- Destination catalog for public APIs/admin management.
-- +goose Up
CREATE TABLE IF NOT EXISTS destinations (
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

CREATE INDEX IF NOT EXISTS idx_destinations_active_sort
ON destinations(is_active, sort_order, id);

INSERT OR IGNORE INTO destinations
    (id, name, code, capital, usd_per_lb, transit_days_min, transit_days_max, is_active, sort_order, created_at, updated_at)
VALUES
    ('guyana', 'Guyana', 'GY', 'Georgetown', 3.50, 3, 5, 1, 10, datetime('now'), datetime('now')),
    ('jamaica', 'Jamaica', 'JM', 'Kingston', 3.75, 3, 5, 1, 20, datetime('now'), datetime('now')),
    ('trinidad', 'Trinidad & Tobago', 'TT', 'Port of Spain', 3.50, 3, 5, 1, 30, datetime('now'), datetime('now')),
    ('barbados', 'Barbados', 'BB', 'Bridgetown', 4.00, 4, 6, 1, 40, datetime('now'), datetime('now')),
    ('suriname', 'Suriname', 'SR', 'Paramaribo', 4.25, 4, 6, 1, 50, datetime('now'), datetime('now'));

-- +goose Down
DROP INDEX IF EXISTS idx_destinations_active_sort;
DROP TABLE IF EXISTS destinations;
