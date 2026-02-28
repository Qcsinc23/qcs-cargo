-- Create observability event record (error/analytics/performance/business).
-- name: CreateObservabilityEvent :exec
INSERT INTO observability_events (
  id,
  category,
  event_name,
  user_id,
  request_id,
  path,
  method,
  status_code,
  duration_ms,
  value,
  metadata_json,
  created_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- Analytics summary for a window.
-- name: ObservabilityAnalyticsSummary :one
SELECT
  COUNT(*) AS total_events,
  COUNT(DISTINCT user_id) AS unique_users,
  COUNT(DISTINCT path) AS unique_routes,
  CAST(
    COALESCE(
      CAST(COUNT(*) AS REAL) / NULLIF(COUNT(DISTINCT user_id), 0),
      0.0
    ) AS REAL
  ) AS avg_events_per_user
FROM observability_events
WHERE category = 'analytics'
  AND created_at >= ?
  AND created_at <= ?;

-- Performance summary for a window with a configurable slow threshold (ms).
-- name: ObservabilityPerformanceSummary :one
SELECT
  COUNT(*) AS total_requests,
  CAST(COALESCE(AVG(duration_ms), 0.0) AS REAL) AS avg_duration_ms,
  CAST(COALESCE(MAX(duration_ms), 0.0) AS REAL) AS max_duration_ms,
  CAST(COALESCE(SUM(CASE WHEN duration_ms >= ? THEN 1 ELSE 0 END), 0) AS INTEGER) AS slow_requests
FROM observability_events
WHERE category = 'performance'
  AND created_at >= ?
  AND created_at <= ?;

-- Top slow routes for a window.
-- name: ObservabilityTopSlowRoutes :many
SELECT
  COALESCE(path, '') AS path,
  COALESCE(method, '') AS method,
  COUNT(*) AS request_count,
  CAST(COALESCE(AVG(duration_ms), 0.0) AS REAL) AS avg_duration_ms,
  CAST(COALESCE(MAX(duration_ms), 0.0) AS REAL) AS max_duration_ms
FROM observability_events
WHERE category = 'performance'
  AND created_at >= ?
  AND created_at <= ?
  AND path IS NOT NULL
GROUP BY path, method
ORDER BY avg_duration_ms DESC, request_count DESC
LIMIT ?;

-- Error summary for a window.
-- name: ObservabilityErrorSummary :one
SELECT
  COUNT(*) AS total_errors,
  COUNT(DISTINCT request_id) AS affected_requests,
  CAST(COALESCE(SUM(CASE WHEN status_code BETWEEN 400 AND 499 THEN 1 ELSE 0 END), 0) AS INTEGER) AS client_errors,
  CAST(COALESCE(SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END), 0) AS INTEGER) AS server_errors
FROM observability_events
WHERE category = 'error'
  AND created_at >= ?
  AND created_at <= ?;

-- Business metric summary for a window.
-- name: ObservabilityBusinessMetricsSummary :one
SELECT
  COUNT(*) AS total_events,
  COUNT(DISTINCT user_id) AS unique_users,
  CAST(COALESCE(SUM(value), 0.0) AS REAL) AS total_value,
  CAST(COALESCE(AVG(value), 0.0) AS REAL) AS avg_value
FROM observability_events
WHERE category = 'business'
  AND created_at >= ?
  AND created_at <= ?;
