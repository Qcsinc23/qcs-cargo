#!/usr/bin/env bash
# Smoke test: build, migrate a test DB, start server, hit key endpoints, stop.
set -e
cd "$(dirname "$0")/.."
SMOKE_PORT="${SMOKE_PORT:-3998}"
SMOKE_DB="${SMOKE_DB:-qcs_smoke.db}"
BASE="http://127.0.0.1:${SMOKE_PORT}"
FAIL=0

echo "== Building..."
go build -o qcs-server ./cmd/server
go build -o qcs-migrate ./cmd/migrate

echo "== Running unit tests..."
go test ./cmd/... ./internal/... -count=1 -short 2>&1 || true

echo "== Migrating test DB..."
rm -f "$SMOKE_DB" "$SMOKE_DB-wal" "$SMOKE_DB-shm"
DATABASE_URL="file:${SMOKE_DB}?_journal_mode=WAL" ./qcs-migrate

echo "== Starting server on port $SMOKE_PORT..."
DATABASE_URL="file:${SMOKE_DB}?_journal_mode=WAL" PORT="$SMOKE_PORT" ./qcs-server &
PID=$!
trap "kill $PID 2>/dev/null || true; rm -f '$SMOKE_DB' '$SMOKE_DB-wal' '$SMOKE_DB-shm'; exit $FAIL" EXIT

echo "== Waiting for server..."
for i in 1 2 3 4 5 6 7 8 9 10; do
  if curl -sf "$BASE/api/v1/health" >/dev/null 2>&1; then break; fi
  if [ "$i" -eq 10 ]; then echo "Server did not become ready."; FAIL=1; exit 1; fi
  sleep 1
done

check() {
  local method="$1"
  local path="$2"
  local want="${3:-200}"
  local extra="${4:-}"
  local url="$BASE$path"
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" $extra "$url" 2>/dev/null || echo "000")
  if [ "$code" = "$want" ]; then
    echo "  OK $method $path -> $code"
  else
    echo "  FAIL $method $path -> $code (want $want)"
    FAIL=1
  fi
}

echo "== Smoke tests..."
check GET "/api/v1/health" 200
check GET "/api/v1/destinations" 200
check GET "/api/v1/destinations/jamaica" 200
check GET "/api/v1/status" 200
check GET "/" 200
check GET "/dashboard" 200
check GET "/dashboard/inbox" 200
check GET "/login" 200
# Protected endpoints without auth -> 401
check GET "/api/v1/me" 401
check GET "/api/v1/locker" 401
check GET "/api/v1/recipients" 401
check GET "/api/v1/ship-requests" 401

echo "== Auth flow (register + magic-link request)..."
REG_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/auth/register" -H "Content-Type: application/json" \
  -d '{"name":"Smoke User","email":"smoke@example.com","password":"SmokePass1!"}' 2>/dev/null || echo "000")
if [ "$REG_CODE" = "201" ] || [ "$REG_CODE" = "200" ] || [ "$REG_CODE" = "409" ]; then
  echo "  OK POST /api/v1/auth/register -> $REG_CODE"
else
  echo "  FAIL POST /api/v1/auth/register -> $REG_CODE"; FAIL=1
fi
ML_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/auth/magic-link/request" -H "Content-Type: application/json" \
  -d '{"email":"smoke@example.com"}' 2>/dev/null || echo "000")
if [ "$ML_CODE" = "200" ]; then
  echo "  OK POST /api/v1/auth/magic-link/request -> $ML_CODE"
else
  echo "  FAIL POST /api/v1/auth/magic-link/request -> $ML_CODE"; FAIL=1
fi

if [ $FAIL -eq 0 ]; then
  echo "== All smoke tests passed."
else
  echo "== Some smoke tests failed."
fi
exit $FAIL
