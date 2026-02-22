-- Warehouse stats: same shape as AdminDashboardCounts.
-- name: WarehouseStats :one
SELECT
  (SELECT COUNT(*) FROM locker_packages) AS locker_packages_count,
  (SELECT COUNT(*) FROM ship_requests) AS ship_requests_count,
  (SELECT COUNT(*) FROM bookings) AS bookings_count,
  (SELECT COUNT(*) FROM service_requests WHERE status = 'pending') AS service_queue_count,
  (SELECT COUNT(*) FROM unmatched_packages WHERE status = 'pending') AS unmatched_count;

-- name: ListServiceQueue :many
SELECT sr.id, sr.user_id, sr.locker_package_id, sr.service_type, sr.status, sr.created_at,
       lp.sender_name, lp.storage_bay, u.name AS user_name
FROM service_requests sr
JOIN locker_packages lp ON sr.locker_package_id = lp.id
JOIN users u ON sr.user_id = u.id
WHERE sr.status = 'pending'
ORDER BY sr.created_at ASC
LIMIT ? OFFSET ?;

-- name: ListShipQueuePaid :many
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, consolidated_weight_lbs, staging_bay, manifest_id,
       created_at, updated_at
FROM ship_requests
WHERE payment_status = 'paid'
ORDER BY created_at ASC
LIMIT ? OFFSET ?;

-- name: ListWarehousePackages :many
SELECT id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE (? = '' OR status = ?)
ORDER BY arrived_at DESC
LIMIT ? OFFSET ?;

-- name: ListWarehouseBays :many
SELECT id, name, zone, destination_id, capacity, current_count
FROM warehouse_bays
ORDER BY zone ASC, name ASC;

-- name: GetWarehouseBayByID :one
SELECT id, name, zone, destination_id, capacity, current_count
FROM warehouse_bays
WHERE id = ?;

-- name: UpdateWarehouseBayCurrentCount :exec
UPDATE warehouse_bays SET current_count = ? WHERE id = ?;

-- name: ListPackagesInBay :many
SELECT id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE storage_bay = ?
ORDER BY arrived_at DESC;

-- name: CreateWarehouseManifest :one
INSERT INTO warehouse_manifests (id, destination_id, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id, destination_id, status, created_at, updated_at;

-- name: ListWarehouseManifests :many
SELECT id, destination_id, status, created_at, updated_at
FROM warehouse_manifests
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetWarehouseManifestByID :one
SELECT id, destination_id, status, created_at, updated_at
FROM warehouse_manifests
WHERE id = ?;

-- name: AddShipRequestToManifest :exec
INSERT INTO warehouse_manifest_ship_requests (manifest_id, ship_request_id) VALUES (?, ?);

-- name: UpdateShipRequestManifestID :exec
UPDATE ship_requests SET manifest_id = ?, updated_at = ? WHERE id = ?;
