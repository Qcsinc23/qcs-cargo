-- Pass 3 D6: drop users.storage_plan.
--
-- The column was provisioned in the initial schema as a forward-looking
-- billing tier flag, but no production code path reads or writes it
-- meaningfully: the API hard-codes 'free' on registration and the
-- column is otherwise dead state. Per Pass 3 D6, we remove it. If a
-- pricing/tier model is reintroduced later, it should live in a
-- dedicated `tiers` table keyed by user_id, not as a denormalized
-- column on users.
--
-- SQLite cannot DROP COLUMN with a NOT NULL/DEFAULT in older versions
-- and we've consistently used the table-rebuild dance elsewhere
-- (see 20260301000000_email_verification.sql, 20260422120000_push_subscriptions_unique_endpoint.sql),
-- so we follow the same pattern here. Indexes recreated below mirror
-- the live state in production at the time of writing.
--
-- goose wraps each migration in a transaction by default, so no
-- explicit BEGIN/COMMIT. The DROP TABLE users + RENAME pair trips
-- foreign_keys=ON because other tables reference users(id); we use
-- PRAGMA defer_foreign_keys=1 (which SQLite honors for the lifetime
-- of the enclosing transaction) so the FK check is deferred until
-- COMMIT. By COMMIT the rebuilt `users` table has the same primary
-- key values, so the FK check passes.

-- +goose Up
PRAGMA defer_foreign_keys = 1;
DROP TABLE IF EXISTS users__new_202604241200;
CREATE TABLE users__new_202604241200 (
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
    free_storage_days INTEGER NOT NULL DEFAULT 30,
    email_verified INTEGER NOT NULL DEFAULT 0,
    email_verification_token TEXT,
    email_verification_sent_at TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO users__new_202604241200 (
    id, name, email, phone, role, avatar_url, password_hash, suite_code,
    address_street, address_city, address_state, address_zip,
    free_storage_days, email_verified,
    email_verification_token, email_verification_sent_at,
    status, created_at, updated_at
)
SELECT
    id, name, email, phone, role, avatar_url, password_hash, suite_code,
    address_street, address_city, address_state, address_zip,
    free_storage_days, email_verified,
    email_verification_token, email_verification_sent_at,
    status, created_at, updated_at
FROM users;

DROP TABLE users;
ALTER TABLE users__new_202604241200 RENAME TO users;

-- Recreate every named index that lived on users prior to the rebuild.
-- Production state confirmed via sqlite_master inspection.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email
    ON users(email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_ci
    ON users(lower(trim(email)));
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_suite_code
    ON users(suite_code)
    WHERE suite_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_role_status
    ON users(role, status);
CREATE INDEX IF NOT EXISTS idx_users_email_verification_token
    ON users(email_verification_token);

-- +goose Down
-- Restore the storage_plan column with its original default. Existing
-- rows will pick up 'free' via the column default.
PRAGMA defer_foreign_keys = 1;
DROP TABLE IF EXISTS users__old_202604241200;
CREATE TABLE users__old_202604241200 (
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
    email_verification_token TEXT,
    email_verification_sent_at TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO users__old_202604241200 (
    id, name, email, phone, role, avatar_url, password_hash, suite_code,
    address_street, address_city, address_state, address_zip,
    free_storage_days, email_verified,
    email_verification_token, email_verification_sent_at,
    status, created_at, updated_at
)
SELECT
    id, name, email, phone, role, avatar_url, password_hash, suite_code,
    address_street, address_city, address_state, address_zip,
    free_storage_days, email_verified,
    email_verification_token, email_verification_sent_at,
    status, created_at, updated_at
FROM users;

DROP TABLE users;
ALTER TABLE users__old_202604241200 RENAME TO users;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email
    ON users(email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_ci
    ON users(lower(trim(email)));
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_suite_code
    ON users(suite_code)
    WHERE suite_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_role_status
    ON users(role, status);
CREATE INDEX IF NOT EXISTS idx_users_email_verification_token
    ON users(email_verification_token);
