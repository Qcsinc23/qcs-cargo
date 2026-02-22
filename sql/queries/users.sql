-- name: GetUserByID :one
SELECT id, name, email, phone, role, avatar_url, suite_code,
       address_street, address_city, address_state, address_zip,
       storage_plan, free_storage_days, email_verified, status, created_at, updated_at
FROM users
WHERE id = ?;

-- name: GetUserByEmail :one
SELECT id, name, email, phone, role, avatar_url, suite_code,
       address_street, address_city, address_state, address_zip,
       storage_plan, free_storage_days, email_verified, status, created_at, updated_at
FROM users
WHERE email = ?;
