-- Storage fee dedupe + integrity guardrails (audit Pass 2 H-7).
-- Adds a UNIQUE(locker_package_id, fee_date) so the storage-fee daily job
-- cannot double-charge under any race or replica scenario, and lets the job
-- use INSERT ... ON CONFLICT DO NOTHING for atomic idempotency.
-- +goose Up
PRAGMA foreign_keys = OFF;

-- Drop any pre-existing duplicate (locker_package_id, fee_date) rows, keeping
-- the earliest by created_at (then id as a tiebreaker). This must run before
-- the unique index is created so the index creation does not fail.
DELETE FROM storage_fees
WHERE id IN (
    SELECT id
    FROM (
        SELECT
            id,
            ROW_NUMBER() OVER (
                PARTITION BY locker_package_id, fee_date
                ORDER BY created_at ASC, id ASC
            ) AS rn
        FROM storage_fees
    )
    WHERE rn > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_fees_pkg_date
    ON storage_fees (locker_package_id, fee_date);

PRAGMA foreign_keys = ON;

-- +goose Down
DROP INDEX IF EXISTS idx_storage_fees_pkg_date;
