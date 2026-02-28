-- INC-011/012/013/014 database hardening for SQLite.
-- +goose Up

-- INC-011: enforce unique non-null suite_code values.
-- Keep earliest user per duplicate suite_code and null out the rest to avoid migration failure.
WITH ranked_suite AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY suite_code
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM users
    WHERE suite_code IS NOT NULL
)
UPDATE users
SET suite_code = NULL
WHERE id IN (SELECT id FROM ranked_suite WHERE rn > 1);

DROP INDEX IF EXISTS idx_users_suite_code;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_suite_code
ON users(suite_code)
WHERE suite_code IS NOT NULL;

-- INC-012: enforce case-insensitive email uniqueness (db-level).
-- If historical duplicates differ only by case/whitespace, keep earliest and rewrite others.
WITH ranked_email AS (
    SELECT
        id,
        lower(trim(email)) AS normalized_email,
        ROW_NUMBER() OVER (
            PARTITION BY lower(trim(email))
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM users
)
UPDATE users
SET email = (
    SELECT
        CASE
            WHEN instr(re.normalized_email, '@') > 1 THEN
                substr(re.normalized_email, 1, instr(re.normalized_email, '@') - 1)
                || '+dedup-' || replace(re.id, '-', '')
                || substr(re.normalized_email, instr(re.normalized_email, '@'))
            ELSE
                re.normalized_email || '+dedup-' || replace(re.id, '-', '')
        END
    FROM ranked_email re
    WHERE re.id = users.id
)
WHERE id IN (SELECT id FROM ranked_email WHERE rn > 1);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_ci
ON users(lower(trim(email)));

-- INC-013: enforce ship_requests status validity.
-- Normalize legacy/invalid statuses before adding CHECK-backed guard.
UPDATE ship_requests
SET status = 'draft'
WHERE status IS NULL
   OR status NOT IN (
       'draft',
       'pending_customs',
       'pending_payment',
       'paid',
       'processing',
       'staged',
       'shipped',
       'delivered',
       'cancelled',
       'expired'
   );

ALTER TABLE ship_requests
ADD COLUMN status_constraint_guard INTEGER NOT NULL DEFAULT 1
CHECK (
    status IN (
        'draft',
        'pending_customs',
        'pending_payment',
        'paid',
        'processing',
        'staged',
        'shipped',
        'delivered',
        'cancelled',
        'expired'
    )
);

-- INC-014: accelerate status-only locker package filtering.
CREATE INDEX IF NOT EXISTS idx_locker_packages_status
ON locker_packages(status);

-- +goose Down
-- Rollback note: data cleanup performed in Up (suite/email dedupe) is not reversible.
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_locker_packages_status;
DROP INDEX IF EXISTS idx_users_email_ci;

-- Rebuild ship_requests to remove status_constraint_guard while preserving prior Phase 4 columns.
DROP TABLE IF EXISTS ship_requests__new_202603010300;
CREATE TABLE ship_requests__new_202603010300 (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    confirmation_code TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    destination_id TEXT NOT NULL,
    recipient_id TEXT,
    service_type TEXT NOT NULL,
    consolidate INTEGER NOT NULL DEFAULT 1,
    special_instructions TEXT,
    subtotal REAL NOT NULL DEFAULT 0,
    service_fees REAL NOT NULL DEFAULT 0,
    insurance REAL NOT NULL DEFAULT 0,
    discount REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    payment_status TEXT,
    stripe_payment_intent_id TEXT,
    customs_status TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    consolidated_weight_lbs REAL,
    staging_bay TEXT,
    manifest_id TEXT
);

INSERT INTO ship_requests__new_202603010300 (
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    created_at, updated_at,
    consolidated_weight_lbs, staging_bay, manifest_id
)
SELECT
    id, user_id, confirmation_code, status, destination_id, recipient_id,
    service_type, consolidate, special_instructions,
    subtotal, service_fees, insurance, discount, total,
    payment_status, stripe_payment_intent_id, customs_status,
    created_at, updated_at,
    consolidated_weight_lbs, staging_bay, manifest_id
FROM ship_requests;

DROP TABLE ship_requests;
ALTER TABLE ship_requests__new_202603010300 RENAME TO ship_requests;

CREATE INDEX IF NOT EXISTS idx_ship_requests_user_status ON ship_requests(user_id, status);
CREATE INDEX IF NOT EXISTS idx_ship_requests_confirmation ON ship_requests(confirmation_code);

DROP INDEX IF EXISTS idx_users_suite_code;
CREATE INDEX IF NOT EXISTS idx_users_suite_code ON users(suite_code);

PRAGMA foreign_keys = ON;
