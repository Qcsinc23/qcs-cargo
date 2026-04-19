-- Pass 3 D6 hotfix2: drop users.storage_plan.
--
-- The column was provisioned in the initial schema as a forward-looking
-- billing tier flag, but no production code path reads or writes it
-- meaningfully. Per Pass 3 D6 we remove it; if a pricing/tier model
-- is reintroduced later it should live in a dedicated `tiers` table
-- keyed by user_id, not as a denormalized column on users.
--
-- Earlier revisions of this migration did the SQLite table-rebuild
-- dance (CREATE users_new; INSERT; DROP TABLE users; RENAME). That
-- failed in production with `FOREIGN KEY constraint failed (787)`
-- because DROP TABLE users trips ON DELETE RESTRICT on the many
-- tables that reference users(id), and PRAGMA defer_foreign_keys
-- only defers DML-level FK checks, not cascaded DDL actions.
--
-- Since modernc.org/sqlite v1.34+ ships SQLite 3.39+ and this schema
-- was always created against SQLite >= 3.35, we use the single-
-- statement `ALTER TABLE ... DROP COLUMN` path. storage_plan has no
-- UNIQUE index, no CHECK constraint, no trigger, no FK reference,
-- and is not part of the PK, so DROP COLUMN is safe.

-- +goose Up
ALTER TABLE users DROP COLUMN storage_plan;

-- +goose Down
ALTER TABLE users ADD COLUMN storage_plan TEXT NOT NULL DEFAULT 'free';
