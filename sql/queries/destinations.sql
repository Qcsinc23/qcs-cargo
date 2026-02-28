-- name: ListActiveDestinations :many
SELECT id, name, code, capital, usd_per_lb, transit_days_min, transit_days_max, is_active, sort_order, created_at, updated_at
FROM destinations
WHERE is_active = 1
ORDER BY sort_order ASC, id ASC;

-- name: GetActiveDestinationByID :one
SELECT id, name, code, capital, usd_per_lb, transit_days_min, transit_days_max, is_active, sort_order, created_at, updated_at
FROM destinations
WHERE id = ? AND is_active = 1;

-- name: ListDestinationsAdmin :many
SELECT id, name, code, capital, usd_per_lb, transit_days_min, transit_days_max, is_active, sort_order, created_at, updated_at
FROM destinations
ORDER BY sort_order ASC, id ASC;

-- name: UpdateDestinationAdmin :exec
UPDATE destinations
SET
    name = ?,
    code = ?,
    capital = ?,
    usd_per_lb = ?,
    transit_days_min = ?,
    transit_days_max = ?,
    is_active = ?,
    sort_order = ?,
    updated_at = ?
WHERE id = ?;

-- name: CountActiveDestinationByID :one
SELECT COUNT(*) AS count
FROM destinations
WHERE id = ? AND is_active = 1;
