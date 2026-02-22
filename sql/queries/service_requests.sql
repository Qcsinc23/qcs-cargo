-- name: CreateServiceRequest :one
INSERT INTO service_requests (id, user_id, locker_package_id, service_type, status, notes, price, created_at)
VALUES (?, ?, ?, ?, 'pending', ?, 0, ?)
RETURNING id;

-- name: ListServiceRequests :many
SELECT id, user_id, locker_package_id, service_type, status, notes, completed_by, price, created_at, completed_at
FROM service_requests
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetServiceRequestByID :one
SELECT id, user_id, locker_package_id, service_type, status, notes, completed_by, price, created_at, completed_at
FROM service_requests
WHERE id = ?;

-- name: ListServiceRequestsByLockerPackageID :many
SELECT id, user_id, locker_package_id, service_type, status, notes, completed_by, price, created_at, completed_at
FROM service_requests
WHERE locker_package_id = ? AND user_id = ?
ORDER BY created_at DESC;

-- name: UpdateServiceRequestStatus :exec
UPDATE service_requests SET status = ?, completed_by = ?, completed_at = ?, price = ? WHERE id = ?;
