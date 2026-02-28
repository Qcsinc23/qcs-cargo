-- +goose Up
ALTER TABLE users ADD COLUMN email_verification_token TEXT;
ALTER TABLE users ADD COLUMN email_verification_sent_at TEXT;
CREATE INDEX IF NOT EXISTS idx_users_email_verification_token ON users(email_verification_token);

-- +goose Down
-- Rebuild users table without email verification columns for SQLite compatibility.
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_users_email_verification_token;

DROP TABLE IF EXISTS users__new_202603010000;
CREATE TABLE users__new_202603010000 (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT,
    role TEXT NOT NULL DEFAULT 'customer',
    avatar_url TEXT,
    password_hash TEXT,
    suite_code TEXT,
    address_street TEXT,
    address_city TEXT,
    address_state TEXT,
    address_zip TEXT,
    storage_plan TEXT NOT NULL DEFAULT 'free',
    free_storage_days INTEGER NOT NULL DEFAULT 30,
    email_verified INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO users__new_202603010000 (
    id, name, email, phone, role, avatar_url, password_hash, suite_code,
    address_street, address_city, address_state, address_zip,
    storage_plan, free_storage_days, email_verified, status,
    created_at, updated_at
)
SELECT
    id, name, email, phone, role, avatar_url, password_hash, suite_code,
    address_street, address_city, address_state, address_zip,
    storage_plan, free_storage_days, email_verified, status,
    created_at, updated_at
FROM users;

DROP TABLE users;
ALTER TABLE users__new_202603010000 RENAME TO users;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_suite_code ON users(suite_code);
CREATE INDEX IF NOT EXISTS idx_users_role_status ON users(role, status);

PRAGMA foreign_keys = ON;
