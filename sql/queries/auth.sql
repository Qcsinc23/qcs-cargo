-- name: CreateUser :one
INSERT INTO users (id, name, email, role, suite_code, storage_plan, free_storage_days, email_verified, status, created_at, updated_at)
VALUES (?, ?, ?, 'customer', ?, 'free', 30, 0, 'active', ?, ?)
RETURNING id, name, email, phone, role, avatar_url, suite_code,
          address_street, address_city, address_state, address_zip,
          storage_plan, free_storage_days, email_verified, status, created_at, updated_at;

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at;

-- name: GetSessionByID :one
SELECT id, user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at
FROM sessions WHERE id = ? AND expires_at > ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: ListSessionsByUser :many
SELECT id, user_id, ip_address, user_agent, expires_at, created_at
FROM sessions
WHERE user_id = ? AND expires_at > ?;

-- name: DeleteSessionsByUserExcept :exec
DELETE FROM sessions WHERE user_id = ? AND id != ?;

-- name: DeleteSessionsByUser :exec
DELETE FROM sessions WHERE user_id = ?;

-- name: CreateMagicLink :one
INSERT INTO magic_links (id, user_id, token_hash, redirect_to, used, expires_at, created_at)
VALUES (?, ?, ?, ?, 0, ?, ?)
RETURNING id, user_id, token_hash, redirect_to, used, expires_at, created_at;

-- name: GetMagicLinkByTokenHash :one
SELECT id, user_id, token_hash, redirect_to, used, expires_at, created_at
FROM magic_links WHERE token_hash = ? AND used = 0 AND expires_at > ?;

-- name: MarkMagicLinkUsed :exec
UPDATE magic_links SET used = 1 WHERE id = ?;

-- name: CreatePasswordReset :one
INSERT INTO password_resets (id, user_id, token_hash, used, expires_at, created_at)
VALUES (?, ?, ?, 0, ?, ?)
RETURNING id, user_id, token_hash, used, expires_at, created_at;

-- name: GetPasswordResetByTokenHash :one
SELECT id, user_id, token_hash, used, expires_at, created_at
FROM password_resets WHERE token_hash = ? AND used = 0 AND expires_at > ?;

-- name: MarkPasswordResetUsed :exec
UPDATE password_resets SET used = 1 WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users SET updated_at = ? WHERE id = ?;

