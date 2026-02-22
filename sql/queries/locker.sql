-- name: ListLockerPackagesByUser :many
SELECT id, user_id, suite_code, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE user_id = ?
  AND (? = '' OR status = ?)
ORDER BY arrived_at DESC;

-- name: GetLockerPackageByID :one
SELECT id, user_id, suite_code, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE id = ? AND user_id = ?;

-- name: LockerSummaryByUser :one
SELECT
  SUM(CASE WHEN status = 'stored' THEN 1 ELSE 0 END) AS stored_count,
  COALESCE(SUM(CASE WHEN status = 'stored' THEN weight_lbs ELSE 0 END), 0) AS stored_weight,
  MIN(CASE WHEN status = 'stored' AND free_storage_expires_at IS NOT NULL THEN free_storage_expires_at END) AS next_expiry,
  SUM(CASE WHEN status = 'service_pending' THEN 1 ELSE 0 END) AS pending_services
FROM locker_packages
WHERE user_id = ?;
