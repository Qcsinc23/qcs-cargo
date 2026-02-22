-- name: CreateServiceRequest :one
INSERT INTO service_requests (id, user_id, locker_package_id, service_type, status, notes, price, created_at)
VALUES (?, ?, ?, ?, 'pending', ?, 0, ?)
RETURNING id;
