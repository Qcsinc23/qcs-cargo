-- +goose Up
CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    used INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    used_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id
    ON email_verification_tokens(user_id);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_expires_at
    ON email_verification_tokens(expires_at);

INSERT INTO email_verification_tokens (id, user_id, token_hash, used, expires_at, created_at, used_at)
SELECT
    lower(hex(randomblob(16))),
    id,
    email_verification_token,
    0,
    CASE
        WHEN email_verification_sent_at IS NOT NULL
            THEN strftime('%Y-%m-%dT%H:%M:%SZ', datetime(email_verification_sent_at, '+24 hours'))
        ELSE strftime('%Y-%m-%dT%H:%M:%SZ', datetime('now', '+24 hours'))
    END,
    COALESCE(email_verification_sent_at, strftime('%Y-%m-%dT%H:%M:%SZ', datetime('now'))),
    NULL
FROM users
WHERE email_verification_token IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM email_verification_tokens evt
      WHERE evt.token_hash = users.email_verification_token
  );

-- +goose Down
DROP INDEX IF EXISTS idx_email_verification_tokens_expires_at;
DROP INDEX IF EXISTS idx_email_verification_tokens_user_id;
DROP TABLE IF EXISTS email_verification_tokens;
