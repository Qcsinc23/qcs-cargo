#!/usr/bin/env bash
# Smoke test: build, migrate a test DB, start server, hit key endpoints. See docs/TESTING_AND_INTEGRATIONS.md.
set -euo pipefail
cd "$(dirname "$0")/.."

SMOKE_PORT="${SMOKE_PORT:-3998}"
SMOKE_DB="${SMOKE_DB:-qcs_smoke.db}"
BASE="http://127.0.0.1:${SMOKE_PORT}"
FAIL=0

# Server needs JWT_SECRET (32+ chars) for token signing; optional fakes for Resend/Stripe so it doesn't log warnings.
export JWT_SECRET="${JWT_SECRET:-smoke-test-secret-key-at-least-32-chars}"
export RESEND_API_KEY="${RESEND_API_KEY:-}"
export STRIPE_SECRET_KEY="${STRIPE_SECRET_KEY:-sk_test_fake}"
export STRIPE_WEBHOOK_SECRET="${STRIPE_WEBHOOK_SECRET:-whsec_fake}"
export FROM_EMAIL="${FROM_EMAIL:-test@qcs-cargo.com}"

echo "=== QCS Cargo Smoke Test ==="

echo "== Building..."
go build -o qcs-server ./cmd/server
go build -o qcs-migrate ./cmd/migrate

echo "== Migrating test DB..."
rm -f "$SMOKE_DB" "$SMOKE_DB-wal" "$SMOKE_DB-shm"
DATABASE_URL="file:${SMOKE_DB}?_journal_mode=WAL" ./qcs-migrate

echo "== Starting server on port $SMOKE_PORT..."
DATABASE_URL="file:${SMOKE_DB}?_journal_mode=WAL" PORT="$SMOKE_PORT" ./qcs-server &
PID=$!
trap "kill $PID 2>/dev/null || true; rm -f '$SMOKE_DB' '$SMOKE_DB-wal' '$SMOKE_DB-shm'; exit $FAIL" EXIT

echo "== Waiting for server..."
for i in 1 2 3 4 5 6 7 8 9 10; do
  if curl -sf "$BASE/api/v1/health" >/dev/null 2>&1; then
    echo "  ✓ Server ready (attempt $i)"
    break
  fi
  if [ "$i" -eq 10 ]; then
    echo "  ✗ Server did not become ready."
    FAIL=1
    exit 1
  fi
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
    echo "  ✓ $method $path -> $code"
  else
    echo "  ✗ $method $path -> $code (want $want)"
    FAIL=1
  fi
}

check_contains() {
  local desc="$1"
  local url="$2"
  local needle="$3"
  local body
  body=$(curl -sf "$url" 2>/dev/null || echo "")
  if echo "$body" | grep -q "$needle"; then
    echo "  ✓ $desc"
  else
    echo "  ✗ $desc (missing '$needle')"
    FAIL=1
  fi
}

echo ""
echo "--- Server health ---"
check GET "/api/v1/health" 200
check_contains "Health returns status" "$BASE/api/v1/health" '"status"'
check_contains "Health DB check" "$BASE/api/v1/health" '"db"'

echo ""
echo "--- App shell ---"
check GET "/" 200
check_contains "Root has app shell" "$BASE/" "QCS Cargo"
# WASM may be referenced from static HTML or loaded by go-app
if curl -sf "$BASE/" | grep -q "app.wasm"; then
  echo "  ✓ Root references app.wasm"
else
  echo "  (skip app.wasm check - not in static HTML)"
fi

echo ""
echo "--- Auth ---"
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/auth/magic-link/request" -H "Content-Type: application/json" -d '{"email":"smoke@test.com","name":"Smoke Test"}')
[ "$code" = "200" ] && echo "  ✓ POST /api/v1/auth/magic-link/request -> 200" || { echo "  ✗ POST /api/v1/auth/magic-link/request -> $code (want 200)"; FAIL=1; }
check POST "/api/v1/auth/magic-link/verify" 400 '' "-H Content-Type: application/json -d '{\"token\":\"invalid-token\"}'"
check GET "/api/v1/locker" 401

REG_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/auth/register" -H "Content-Type: application/json" \
  -d '{"name":"Smoke User","email":"smoke@example.com"}' 2>/dev/null || echo "000")
if [ "$REG_CODE" = "201" ] || [ "$REG_CODE" = "200" ] || [ "$REG_CODE" = "409" ]; then
  echo "  ✓ POST /api/v1/auth/register -> $REG_CODE"
else
  echo "  ✗ POST /api/v1/auth/register -> $REG_CODE"; FAIL=1
fi

echo ""
echo "--- Public routes ---"
check GET "/api/v1/destinations" 200
check GET "/api/v1/destinations/guyana" 200
check GET "/api/v1/calculator?dest=guyana&weight=5" 200
check GET "/api/v1/status" 200
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/contact" -H "Content-Type: application/json" -d '{"name":"Test","email":"test@test.com","subject":"Other","message":"Smoke test message"}')
[ "$code" = "200" ] && echo "  ✓ POST /api/v1/contact -> 200" || { echo "  ✗ POST /api/v1/contact -> $code (want 200)"; FAIL=1; }

echo ""
echo "--- Protected routes (no auth) ---"
check GET "/api/v1/me" 401
check GET "/api/v1/recipients" 401
check GET "/api/v1/ship-requests" 401

echo ""
echo "--- Static / dashboard ---"
check GET "/dashboard" 200
check GET "/dashboard/inbox" 200
check GET "/login" 200

echo ""
echo "--- Error handling ---"
check GET "/api/v1/nonexistent" 404
check GET "/api/v1/track/FAKE-000" 404

echo ""
echo "==========================="
if [ $FAIL -eq 0 ]; then
  echo "All smoke tests passed."
else
  echo "Some smoke tests failed."
fi
echo "==========================="
exit $FAIL
