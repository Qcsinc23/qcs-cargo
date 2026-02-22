-- Admin dashboard KPIs. PRD 6.9 GET /admin/dashboard.
-- name: AdminDashboardCounts :one
SELECT
  (SELECT COUNT(*) FROM locker_packages) AS locker_packages_count,
  (SELECT COUNT(*) FROM ship_requests) AS ship_requests_count,
  (SELECT COUNT(*) FROM bookings) AS bookings_count,
  (SELECT COUNT(*) FROM service_requests WHERE status = 'pending') AS service_queue_count,
  (SELECT COUNT(*) FROM unmatched_packages WHERE status = 'pending') AS unmatched_count;

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

-- Global search: users by name, email, or suite_code. Limit 5.
-- name: AdminSearchUsers :many
SELECT id, name, email, phone, role, avatar_url, suite_code,
       address_street, address_city, address_state, address_zip,
       storage_plan, free_storage_days, email_verified, status, created_at, updated_at
FROM users
WHERE name LIKE ? OR email LIKE ? OR (suite_code IS NOT NULL AND suite_code LIKE ?)
ORDER BY name
LIMIT 5;

-- Global search: ship_requests by confirmation_code. Limit 5.
-- name: AdminSearchShipRequests :many
SELECT id, user_id, confirmation_code, status, destination_id, recipient_id, service_type,
       consolidate, special_instructions, subtotal, service_fees, insurance, discount, total,
       payment_status, stripe_payment_intent_id, customs_status, created_at, updated_at
FROM ship_requests
WHERE confirmation_code LIKE ?
ORDER BY created_at DESC
LIMIT 5;

-- Global search: locker_packages by suite_code or sender_name. Limit 5.
-- name: AdminSearchLockerPackages :many
SELECT id, user_id, suite_code, tracking_inbound, carrier_inbound, sender_name, sender_address,
       weight_lbs, length_in, width_in, height_in, arrival_photo_url, condition, storage_bay,
       status, arrived_at, free_storage_expires_at, disposed_at, created_at, updated_at
FROM locker_packages
WHERE suite_code LIKE ? OR (sender_name IS NOT NULL AND sender_name LIKE ?)
ORDER BY arrived_at DESC
LIMIT 5;
