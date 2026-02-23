-- name: CreateLockerPackage :one
INSERT INTO locker_packages (
    id, user_id, suite_code, tracking_inbound, carrier_inbound, sender_name,
    weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
    status, arrived_at, free_storage_expires_at, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'stored', ?, ?, ?, ?
)
RETURNING id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
          weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
          status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at;

-- name: CreateLockerPackageFromBooking :one
INSERT INTO locker_packages (
    id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name,
    weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
    status, arrived_at, free_storage_expires_at, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'stored', ?, ?, ?, ?
)
RETURNING id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
          weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
          status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at;

-- name: ListLockerPackagesByUser :many
SELECT id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE user_id = ?
  AND (? = '' OR status = ?)
ORDER BY arrived_at DESC;

-- name: GetLockerPackageByID :one
SELECT id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE id = ? AND user_id = ?;

-- name: GetLockerPackageByIDOnly :one
SELECT id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE id = ?;

-- name: LockerSummaryByUser :one
SELECT
  SUM(CASE WHEN status = 'stored' THEN 1 ELSE 0 END) AS stored_count,
  COALESCE(SUM(CASE WHEN status = 'stored' THEN weight_lbs ELSE 0 END), 0) AS stored_weight,
  MIN(CASE WHEN status = 'stored' AND free_storage_expires_at IS NOT NULL THEN free_storage_expires_at END) AS next_expiry,
  SUM(CASE WHEN status = 'service_pending' THEN 1 ELSE 0 END) AS pending_services
FROM locker_packages
WHERE user_id = ?;

-- name: AdminListLockerPackages :many
SELECT id, user_id, suite_code, booking_id, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE (? = '' OR user_id = ?)
  AND (? = '' OR suite_code = ?)
  AND (? = '' OR status = ?)
ORDER BY arrived_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateLockerPackageStorageBay :exec
UPDATE locker_packages SET storage_bay = ?, updated_at = ? WHERE id = ?;
-- name: UpdateLockerPackageStatus :exec
UPDATE locker_packages SET status = ?, updated_at = ? WHERE id = ?;
