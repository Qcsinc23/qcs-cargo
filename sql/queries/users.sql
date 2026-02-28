-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;

-- name: GetUserBySuiteCode :one
SELECT * FROM users WHERE suite_code = ?;

-- name: UpdateUserProfile :exec
UPDATE users
SET name = ?, phone = ?, address_street = ?, address_city = ?, address_state = ?, address_zip = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_url = ?, updated_at = ? WHERE id = ?;

-- name: UpdateUserStatus :exec
UPDATE users SET status = ?, updated_at = ? WHERE id = ?;

-- name: AnonymizeUserForDeletion :exec
UPDATE users
SET
  name = ?,
  email = ?,
  phone = NULL,
  avatar_url = NULL,
  password_hash = NULL,
  suite_code = NULL,
  address_street = NULL,
  address_city = NULL,
  address_state = NULL,
  address_zip = NULL,
  email_verified = 0,
  email_verification_token = NULL,
  email_verification_sent_at = NULL,
  status = 'deleted',
  updated_at = ?
WHERE id = ?;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateUserRole :exec
UPDATE users SET role = ?, updated_at = ? WHERE id = ?;

-- name: UpdateUserRoleAndStatus :exec
UPDATE users SET role = ?, status = ?, updated_at = ? WHERE id = ?;
