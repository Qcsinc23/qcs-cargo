CREATE TABLE email_verification_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    used INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    used_at TEXT
);

CREATE INDEX idx_email_verification_tokens_user_id
    ON email_verification_tokens(user_id);

CREATE INDEX idx_email_verification_tokens_expires_at
    ON email_verification_tokens(expires_at);
