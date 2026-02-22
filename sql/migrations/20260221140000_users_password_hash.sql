-- Add password_hash for password reset and optional password login (PRD 6.1).
-- +goose Up
ALTER TABLE users ADD COLUMN password_hash TEXT;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; use a new table if rollback needed.
-- For simplicity we leave the column on rollback or use a no-op:
-- ALTER TABLE users DROP COLUMN password_hash;
