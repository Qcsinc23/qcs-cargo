-- Observability events for error tracking, analytics, APM, and business metrics.
-- +goose Up
CREATE TABLE IF NOT EXISTS observability_events (
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

CREATE INDEX IF NOT EXISTS idx_observability_events_category_created
ON observability_events(category, created_at);

CREATE INDEX IF NOT EXISTS idx_observability_events_event_created
ON observability_events(event_name, created_at);

CREATE INDEX IF NOT EXISTS idx_observability_events_user_created
ON observability_events(user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_observability_events_path_created
ON observability_events(path, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_observability_events_path_created;
DROP INDEX IF EXISTS idx_observability_events_user_created;
DROP INDEX IF EXISTS idx_observability_events_event_created;
DROP INDEX IF EXISTS idx_observability_events_category_created;
DROP TABLE IF EXISTS observability_events;
