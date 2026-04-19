-- name: ListShipmentsByUser :many
-- Pass 3 HIGH-07: real SQL LIMIT/OFFSET pagination.
SELECT s.id, s.destination_id, s.manifest_id, s.ship_request_id, s.tracking_number, s.status,
       s.total_weight, s.package_count, s.carrier, s.estimated_delivery, s.actual_delivery,
       s.created_at, s.updated_at
FROM shipments s
INNER JOIN ship_requests sr ON s.ship_request_id = sr.id
WHERE sr.user_id = ?
ORDER BY s.created_at DESC
LIMIT ? OFFSET ?;

-- name: CountShipmentsByUser :one
SELECT COUNT(*)
FROM shipments s
INNER JOIN ship_requests sr ON s.ship_request_id = sr.id
WHERE sr.user_id = ?;

-- name: GetShipmentByID :one
SELECT s.id, s.destination_id, s.manifest_id, s.ship_request_id, s.tracking_number, s.status,
       s.total_weight, s.package_count, s.carrier, s.estimated_delivery, s.actual_delivery,
       s.created_at, s.updated_at
FROM shipments s
INNER JOIN ship_requests sr ON s.ship_request_id = sr.id
WHERE s.id = ? AND sr.user_id = ?;
