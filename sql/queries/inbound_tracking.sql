-- name: ListInboundTrackingByUser :many
SELECT id, user_id, carrier, tracking_number, retailer_name, expected_items,
       status, locker_package_id, last_checked_at, created_at
FROM inbound_tracking
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: GetInboundTrackingByID :one
SELECT id, user_id, carrier, tracking_number, retailer_name, expected_items,
       status, locker_package_id, last_checked_at, created_at
FROM inbound_tracking
WHERE id = ? AND user_id = ?;

-- name: CreateInboundTracking :one
INSERT INTO inbound_tracking (id, user_id, carrier, tracking_number, retailer_name, expected_items, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, 'tracking', ?)
RETURNING id;

-- name: DeleteInboundTracking :exec
DELETE FROM inbound_tracking
WHERE id = ? AND user_id = ?;
