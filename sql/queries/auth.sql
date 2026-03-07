-- name: CreateUser :one
INSERT INTO users (id, name, email, phone, password_hash, role, suite_code, storage_plan, free_storage_days, email_verified, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 'customer', ?, 'free', 30, 0, 'active', ?, ?)
RETURNING *;

-- name: SetEmailVerificationToken :exec
UPDATE users
SET email_verification_token = ?, email_verification_sent_at = ?, updated_at = ?
WHERE id = ?;

-- name: GetUserByEmailVerificationToken :one
SELECT * FROM users
WHERE email_verification_token = ?;

-- name: SetEmailVerified :exec
UPDATE users
SET email_verified = 1,
    email_verification_token = NULL,
    email_verification_sent_at = NULL,
    updated_at = ?
WHERE id = ?;

-- name: CreateEmailVerificationToken :one
INSERT INTO email_verification_tokens (id, user_id, token_hash, used, expires_at, created_at, used_at)
VALUES (?, ?, ?, 0, ?, ?, NULL)
RETURNING id, user_id, token_hash, used, expires_at, created_at, used_at;

-- name: GetEmailVerificationTokenByHash :one
SELECT id, user_id, token_hash, used, expires_at, created_at, used_at
FROM email_verification_tokens
WHERE token_hash = ?;

-- name: MarkEmailVerificationTokenUsed :exec
UPDATE email_verification_tokens
SET used = 1, used_at = ?
WHERE id = ?;

-- name: MarkEmailVerificationTokensUsedByUser :exec
UPDATE email_verification_tokens
SET used = 1, used_at = ?
WHERE user_id = ? AND used = 0;

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

-- name: CreateTokenBlacklist :exec
INSERT INTO token_blacklist (id, token_jti, expires_at, created_at)
VALUES (?, ?, ?, ?);

-- name: CountTokenBlacklistByJti :one
SELECT COUNT(*)
FROM token_blacklist
WHERE token_jti = ? AND expires_at > ?;

-- name: DeleteExpiredTokenBlacklist :exec
DELETE FROM token_blacklist
WHERE expires_at <= ?;
