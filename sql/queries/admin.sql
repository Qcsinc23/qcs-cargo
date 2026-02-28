-- Admin dashboard KPIs. PRD 6.9 GET /admin/dashboard.
-- name: AdminDashboardCounts :one
SELECT
  (SELECT COUNT(*) FROM locker_packages) AS locker_packages_count,
  (SELECT COUNT(*) FROM ship_requests) AS ship_requests_count,
  (SELECT COUNT(*) FROM bookings) AS bookings_count,
  (SELECT COUNT(*) FROM service_requests WHERE status = 'pending') AS service_queue_count,
  (SELECT COUNT(*) FROM unmatched_packages WHERE status = 'pending') AS unmatched_count;

-- Admin system-health snapshot counters.
-- name: AdminSystemHealthCounts :one
SELECT
  (SELECT COUNT(*) FROM users) AS users_count,
  (SELECT COUNT(*) FROM locker_packages) AS locker_packages_count,
  (SELECT COUNT(*) FROM ship_requests) AS ship_requests_count,
  (SELECT COUNT(*) FROM bookings) AS bookings_count,
  (SELECT COUNT(*) FROM service_requests WHERE status = 'pending') AS pending_service_requests,
  (SELECT COUNT(*) FROM ship_requests WHERE status IN ('pending_customs', 'pending_payment', 'paid', 'processing', 'staged')) AS pending_ship_requests,
  (SELECT COUNT(*) FROM unmatched_packages WHERE status = 'pending') AS unmatched_pending_count;

-- Storage report: PRD Phase 3 GET /api/v1/admin/storage-report
-- name: AdminStorageReport :one
SELECT
  (SELECT COUNT(*) FROM locker_packages WHERE status = 'stored') AS total_packages_stored,
  (SELECT COALESCE(SUM(weight_lbs), 0) FROM locker_packages WHERE status = 'stored' AND weight_lbs IS NOT NULL) AS total_weight,
  (SELECT COUNT(*) FROM locker_packages WHERE status = 'stored' AND free_storage_expires_at IS NOT NULL
   AND date(free_storage_expires_at) BETWEEN date('now') AND date('now', '+5 days')) AS packages_expiring_soon,
  (SELECT COALESCE(SUM(amount), 0) FROM storage_fees WHERE date(fee_date) = date('now')) AS storage_fees_collected_today;

-- Revenue report by period: sum ship_requests.total where status in ('shipped','paid')
-- name: AdminRevenueReport :one
SELECT COALESCE(SUM(total), 0) AS revenue
FROM ship_requests
WHERE status IN ('shipped', 'paid')
  AND (created_at >= ? OR ? = '')
  AND (created_at <= ? OR ? = '');

-- Shipments count by period (ship_requests with status shipped/paid, or count from shipments table)
-- name: AdminShipmentsCountReport :one
SELECT COUNT(*) AS count
FROM ship_requests
WHERE status IN ('shipped', 'paid')
  AND (created_at >= ? OR ? = '')
  AND (created_at <= ? OR ? = '');

-- Customers count (total users with role customer, or new signups if from/to provided)
-- name: AdminCustomersCount :one
SELECT COUNT(*) AS count FROM users WHERE role = 'customer';

-- name: AdminNewSignupsCount :one
SELECT COUNT(*) AS count FROM users WHERE role = 'customer' AND (created_at >= ? OR ? = '') AND (created_at <= ? OR ? = '');

-- Pending ship requests (awaiting payment). For dashboard pending actions.
-- name: AdminDashboardPendingShipCount :one
SELECT COUNT(*) AS count FROM ship_requests
WHERE payment_status IS NULL OR payment_status != 'paid';

-- System health snapshot for admin monitoring dashboard.
-- name: AdminSystemHealthSnapshot :one
SELECT
  (SELECT COUNT(*) FROM users) AS users_count,
  (SELECT COUNT(*) FROM locker_packages) AS locker_packages_count,
  (SELECT COUNT(*) FROM locker_packages WHERE status = 'stored') AS stored_packages_count,
  (SELECT COUNT(*) FROM service_requests WHERE status = 'pending') AS pending_service_requests_count,
  (SELECT COUNT(*) FROM unmatched_packages WHERE status = 'pending') AS pending_unmatched_packages_count,
  (SELECT COUNT(*) FROM ship_requests WHERE payment_status IS NULL OR payment_status != 'paid') AS pending_ship_requests_count;

-- Global search: users by name, email, or suite_code. Limit 5.
-- name: AdminSearchUsers :many
SELECT * FROM users
WHERE name LIKE ? OR email LIKE ? OR (suite_code IS NOT NULL AND suite_code LIKE ?)
ORDER BY name
LIMIT ? OFFSET ?;

-- Global search: ship_requests by confirmation_code. Limit 5.
-- name: AdminSearchShipRequests :many
SELECT *
FROM ship_requests
WHERE confirmation_code LIKE ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- Global search: locker_packages by suite_code or sender_name. Limit 5.
-- name: AdminSearchLockerPackages :many
SELECT id, user_id, suite_code, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE suite_code LIKE ? OR (sender_name IS NOT NULL AND sender_name LIKE ?)
ORDER BY arrived_at DESC
LIMIT ? OFFSET ?;
