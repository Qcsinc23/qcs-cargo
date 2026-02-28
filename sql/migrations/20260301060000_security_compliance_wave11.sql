-- Security/compliance wave 11 foundations (MISS-002/021/022/023/032/033/035/036).
-- +goose Up

CREATE TABLE IF NOT EXISTS user_mfa (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
    method TEXT NOT NULL DEFAULT 'email_otp' CHECK (method IN ('email_otp', 'totp')),
    secret TEXT,
    otp_code_hash TEXT,
    otp_expires_at TEXT,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 0,
    last_challenge_at TEXT,
    last_verified_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_user_mfa_user ON user_mfa(user_id);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    scopes_json TEXT NOT NULL DEFAULT '[]',
    last_used_at TEXT,
    expires_at TEXT,
    revoked_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(key_hash)
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id, revoked_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);

CREATE TABLE IF NOT EXISTS ip_access_rules (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    api_key_id TEXT REFERENCES api_keys(id),
    cidr TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('allow', 'deny')),
    enabled INTEGER NOT NULL DEFAULT 1,
    description TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (user_id IS NOT NULL OR api_key_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_ip_access_rules_api_key ON ip_access_rules(api_key_id, enabled);
CREATE INDEX IF NOT EXISTS idx_ip_access_rules_user ON ip_access_rules(user_id, enabled);

CREATE TABLE IF NOT EXISTS feature_flags (
    flag_key TEXT PRIMARY KEY,
    description TEXT,
    enabled INTEGER NOT NULL DEFAULT 0,
    rollout_percent INTEGER NOT NULL DEFAULT 100 CHECK (rollout_percent >= 0 AND rollout_percent <= 100),
    updated_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_feature_flags_enabled ON feature_flags(enabled, flag_key);

CREATE TABLE IF NOT EXISTS cookie_consents (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
    consent_version TEXT NOT NULL,
    necessary INTEGER NOT NULL DEFAULT 1,
    functional INTEGER NOT NULL DEFAULT 0,
    analytics INTEGER NOT NULL DEFAULT 0,
    marketing INTEGER NOT NULL DEFAULT 0,
    source TEXT,
    ip_address TEXT,
    user_agent TEXT,
    consented_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cookie_consents_user ON cookie_consents(user_id);

CREATE TABLE IF NOT EXISTS resource_versions (
    id TEXT PRIMARY KEY,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    version_no INTEGER NOT NULL,
    change_type TEXT NOT NULL,
    data_json TEXT NOT NULL DEFAULT '{}',
    changed_by TEXT,
    created_at TEXT NOT NULL,
    UNIQUE(resource_type, resource_id, version_no)
);
CREATE INDEX IF NOT EXISTS idx_resource_versions_lookup
ON resource_versions(resource_type, resource_id, created_at);
CREATE INDEX IF NOT EXISTS idx_resource_versions_changed_by
ON resource_versions(changed_by, created_at);

ALTER TABLE recipients ADD COLUMN deleted_at TEXT;
CREATE INDEX IF NOT EXISTS idx_recipients_user_deleted_at ON recipients(user_id, deleted_at);

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_recipients_user_deleted_at;

DROP TABLE IF EXISTS recipients__new_202603010600;
CREATE TABLE recipients__new_202603010600 (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    phone TEXT,
    destination_id TEXT NOT NULL,
    street TEXT NOT NULL,
    apt TEXT,
    city TEXT NOT NULL,
    delivery_instructions TEXT,
    is_default INTEGER NOT NULL DEFAULT 0,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO recipients__new_202603010600 (
    id, user_id, name, phone, destination_id, street, apt, city,
    delivery_instructions, is_default, use_count, created_at, updated_at
)
SELECT
    id, user_id, name, phone, destination_id, street, apt, city,
    delivery_instructions, is_default, use_count, created_at, updated_at
FROM recipients;

DROP TABLE recipients;
ALTER TABLE recipients__new_202603010600 RENAME TO recipients;

DROP INDEX IF EXISTS idx_resource_versions_changed_by;
DROP INDEX IF EXISTS idx_resource_versions_lookup;
DROP TABLE IF EXISTS resource_versions;

DROP INDEX IF EXISTS idx_cookie_consents_user;
DROP TABLE IF EXISTS cookie_consents;

DROP INDEX IF EXISTS idx_feature_flags_enabled;
DROP TABLE IF EXISTS feature_flags;

DROP INDEX IF EXISTS idx_ip_access_rules_user;
DROP INDEX IF EXISTS idx_ip_access_rules_api_key;
DROP TABLE IF EXISTS ip_access_rules;

DROP INDEX IF EXISTS idx_api_keys_hash;
DROP INDEX IF EXISTS idx_api_keys_user;
DROP TABLE IF EXISTS api_keys;

DROP INDEX IF EXISTS idx_user_mfa_user;
DROP TABLE IF EXISTS user_mfa;

PRAGMA foreign_keys = ON;
