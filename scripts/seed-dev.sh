#!/usr/bin/env bash
# Dev seed: run migrations then insert admin, staff, and customer users.
# Requires: qcs-migrate (or go run ./cmd/migrate), sqlite3.
# Usage: ./scripts/seed-dev.sh   (uses file:qcs.db if DATABASE_URL unset)
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -z "${DATABASE_URL:-}" ]; then
  export DATABASE_URL="file:qcs.db"
fi

# Resolve DB file path for sqlite3 (strip "file:" and any ?query)
DB_FILE="${DATABASE_URL#file:}"
DB_FILE="${DB_FILE%%\?*}"

echo "=== QCS Cargo dev seed ==="
echo "DATABASE_URL=$DATABASE_URL -> DB file: $DB_FILE"

echo "== Running migrations..."
if [ -x "./qcs-migrate" ]; then
  ./qcs-migrate
else
  go run ./cmd/migrate
fi

echo "== Seeding users..."
sqlite3 "$DB_FILE" <<'SQL'
INSERT OR REPLACE INTO users (id, name, email, phone, role, suite_code, storage_plan, free_storage_days, email_verified, status, created_at, updated_at)
VALUES
  ('usr_dev_admin',    'Dev Admin',    'admin@qcs-cargo.local',    NULL, 'admin',  NULL,        'free', 30, 1, 'active', datetime('now'), datetime('now')),
  ('usr_dev_staff',    'Dev Staff',    'staff@qcs-cargo.local',    NULL, 'staff',  NULL,        'free', 30, 1, 'active', datetime('now'), datetime('now')),
  ('usr_dev_customer', 'Dev Customer', 'customer@qcs-cargo.local', NULL, 'customer', 'QCS-DEV01', 'free', 30, 1, 'active', datetime('now'), datetime('now'));
SQL

echo "  ✓ admin@qcs-cargo.local (admin)"
echo "  ✓ staff@qcs-cargo.local (staff)"
echo "  ✓ customer@qcs-cargo.local (customer, suite_code QCS-DEV01)"
echo "=== Done ==="
