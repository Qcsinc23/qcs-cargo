# QCS Cargo — Testing & Integration Strategy

**Purpose:** This document provides exact implementation instructions for the QCS Cargo testing infrastructure. It is designed to be consumed by an AI coding agent building the application defined in the Unified PRD v3.0.

**Stack context:** Go 1.25+ · Fiber v2 · go-app (WASM) · modernc.org/sqlite · sqlc · goose · Stripe · Resend · Playwright

**Repo note:** When implementing code from this doc, use module path `github.com/Qcsinc23/qcs-cargo` (replace any `github.com/qcs-cargo/app` in examples). Reference this file when adding integration tests, CI, E2E, or test data seeding.

---

## Table of Contents

1. [CI Pipeline Configuration](#1-ci-pipeline-configuration)
2. [Test Data Seeding](#2-test-data-seeding)
3. [Unit Testing Strategy](#3-unit-testing-strategy)
4. [Integration Testing (SQLite In-Memory)](#4-integration-testing)
5. [Offline Warehouse E2E Testing](#5-offline-warehouse-e2e-testing)
6. [Stripe Payment Testing](#6-stripe-payment-testing)
7. [Storage Fee Cron Job Testing](#7-storage-fee-cron-job-testing)
8. [Load Testing](#8-load-testing)
9. [go-app Component Testing](#9-go-app-component-testing)
10. [Test File Organization](#10-test-file-organization)

---

## 1. CI Pipeline Configuration

### 1.1 Full `.github/workflows/ci.yml`

Replace the existing CI workflow with this complete version. It adds smoke testing and E2E testing jobs to the existing lint/test/build pipeline.

```yaml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

env:
  GO_VERSION: '1.22'

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

  test-unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      - name: Run unit tests
        run: go test ./internal/... -race -cover -short -coverprofile=coverage.out
      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: ${COVERAGE}%"
          if (( $(echo "$COVERAGE < 60" | bc -l) )); then
            echo "::error::Coverage ${COVERAGE}% is below 60% threshold"
            exit 1
          fi

  test-integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      - name: Run integration tests
        run: go test ./internal/api/... -race -count=1 -tags=integration
        env:
          DATABASE_URL: "file::memory:?cache=shared"
          JWT_SECRET: "test-secret-key-for-ci-do-not-use-in-production"

  test-integration-wal:
    runs-on: ubuntu-latest
    # Run nightly, not on every push
    if: github.event_name == 'schedule' || contains(github.event.head_commit.message, '[test-wal]')
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      - name: Run WAL-mode integration tests
        run: |
          mkdir -p /tmp/qcs-test
          go test ./internal/api/... -race -count=1 -tags=integration,wal \
            -timeout 120s
        env:
          DATABASE_URL: "/tmp/qcs-test/test.db?_journal_mode=WAL"
          JWT_SECRET: "test-secret-key-for-ci-do-not-use-in-production"

  build-server:
    runs-on: ubuntu-latest
    needs: [lint, test-unit]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
      - name: Build server binary
        run: go build -o qcs-server ./cmd/server
      - name: Build WASM
        run: GOOS=js GOARCH=wasm go build -o web/app.wasm ./frontend
      - name: Check WASM size
        run: |
          SIZE=$(stat -f%z web/app.wasm 2>/dev/null || stat -c%s web/app.wasm)
          SIZE_MB=$(echo "scale=2; $SIZE / 1048576" | bc)
          echo "WASM size: ${SIZE_MB} MB"
          if (( $(echo "$SIZE > 12582912" | bc -l) )); then
            echo "::warning::WASM binary exceeds 12MB (${SIZE_MB}MB)"
          fi
      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: build
          path: |
            qcs-server
            web/app.wasm

  smoke-test:
    runs-on: ubuntu-latest
    needs: [build-server]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          name: build
      - name: Make binary executable
        run: chmod +x qcs-server
      - name: Run smoke test
        run: |
          bash scripts/smoke-test.sh
        env:
          DATABASE_URL: "/tmp/qcs-smoke/test.db"
          JWT_SECRET: "smoke-test-secret"
          PORT: "3998"
          RESEND_API_KEY: "re_test_fake"
          STRIPE_SECRET_KEY: "sk_test_fake"
          STRIPE_WEBHOOK_SECRET: "whsec_test_fake"
          FROM_EMAIL: "test@qcs-cargo.com"
          APP_URL: "http://localhost:3998"

  e2e:
    runs-on: ubuntu-latest
    needs: [smoke-test]
    if: github.ref == 'refs/heads/main' || contains(github.event.head_commit.message, '[e2e]')
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          name: build
      - name: Make binary executable
        run: chmod +x qcs-server
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - name: Install Playwright
        run: cd e2e && npm ci && npx playwright install --with-deps chromium
      - name: Start server
        run: |
          chmod +x qcs-server
          ./qcs-server &
          sleep 3
          curl -sf http://localhost:8080/api/v1/health || exit 1
        env:
          DATABASE_URL: "/tmp/qcs-e2e/test.db"
          JWT_SECRET: "e2e-test-secret"
          RESEND_API_KEY: "re_test_fake"
          STRIPE_SECRET_KEY: "sk_test_fake"
          STRIPE_WEBHOOK_SECRET: "whsec_test_fake"
          FROM_EMAIL: "test@qcs-cargo.com"
          APP_URL: "http://localhost:8080"
      - name: Run E2E tests
        run: cd e2e && npx playwright test
      - name: Upload test results
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: e2e/playwright-report/
```

### 1.2 Smoke Test Script

Create `scripts/smoke-test.sh`. This verifies the server boots and critical paths respond.

```bash
#!/usr/bin/env bash
set -euo pipefail

PORT="${PORT:-3998}"
DB_DIR="/tmp/qcs-smoke"
BASE="http://localhost:${PORT}"

echo "=== QCS Cargo Smoke Test ==="

# Setup
rm -rf "$DB_DIR" && mkdir -p "$DB_DIR"

# Start server in background
./qcs-server &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null; rm -rf $DB_DIR" EXIT

# Wait for server to be ready (max 10 seconds)
for i in $(seq 1 20); do
  if curl -sf "${BASE}/api/v1/health" > /dev/null 2>&1; then
    echo "✓ Server started (attempt $i)"
    break
  fi
  if [ "$i" -eq 20 ]; then
    echo "✗ Server failed to start"
    exit 1
  fi
  sleep 0.5
done

PASS=0
FAIL=0

check() {
  local desc="$1"
  local url="$2"
  local expected_status="${3:-200}"
  local method="${4:-GET}"
  local body="${5:-}"

  if [ -n "$body" ]; then
    STATUS=$(curl -sf -o /dev/null -w "%{http_code}" -X "$method" \
      -H "Content-Type: application/json" -d "$body" "$url" 2>/dev/null || echo "000")
  else
    STATUS=$(curl -sf -o /dev/null -w "%{http_code}" -X "$method" "$url" 2>/dev/null || echo "000")
  fi

  if [ "$STATUS" -eq "$expected_status" ]; then
    echo "  ✓ $desc (HTTP $STATUS)"
    PASS=$((PASS + 1))
  else
    echo "  ✗ $desc (expected $expected_status, got $STATUS)"
    FAIL=$((FAIL + 1))
  fi
}

check_contains() {
  local desc="$1"
  local url="$2"
  local needle="$3"

  BODY=$(curl -sf "$url" 2>/dev/null || echo "")
  if echo "$BODY" | grep -q "$needle"; then
    echo "  ✓ $desc (contains '$needle')"
    PASS=$((PASS + 1))
  else
    echo "  ✗ $desc (missing '$needle')"
    FAIL=$((FAIL + 1))
  fi
}

# --- Critical Path: Server Health ---
echo ""
echo "--- Server Health ---"
check "Health endpoint" "${BASE}/api/v1/health"
check_contains "Health returns OK" "${BASE}/api/v1/health" '"status":"ok"'
check_contains "Health DB check" "${BASE}/api/v1/health" '"db":"ok"'

# --- Critical Path: WASM App Shell ---
echo ""
echo "--- WASM App Shell ---"
check "Root serves HTML" "${BASE}/"
check_contains "Root loads WASM" "${BASE}/" "app.wasm"
check_contains "Root has app shell" "${BASE}/" "QCS Cargo"

# --- Critical Path: Auth ---
echo ""
echo "--- Authentication ---"
check "Magic link request" "${BASE}/api/v1/auth/magic-link/request" 200 POST \
  '{"email":"smoke@test.com","name":"Smoke Test"}'
check "Invalid magic link verify" "${BASE}/api/v1/auth/magic-link/verify" 400 POST \
  '{"token":"invalid-token"}'
check "Unauthenticated locker" "${BASE}/api/v1/locker" 401

# --- Critical Path: Public Routes ---
echo ""
echo "--- Public Routes ---"
check "Destinations" "${BASE}/api/v1/destinations"
check "Calculator" "${BASE}/api/v1/calculator?destination=guyana&service=standard&weight=5"
check "System status" "${BASE}/api/v1/status"
check "Contact form" "${BASE}/api/v1/contact" 200 POST \
  '{"name":"Test","email":"test@test.com","subject":"Other","message":"Smoke test message for validation"}'

# --- Critical Path: 404 Handling ---
echo ""
echo "--- Error Handling ---"
check "API 404" "${BASE}/api/v1/nonexistent" 404
check "Track invalid number" "${BASE}/api/v1/track/FAKE-000" 404

# --- Results ---
echo ""
echo "==========================="
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo "==========================="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi

echo "All smoke tests passed!"
```

### 1.3 Makefile Targets

Add these targets to your `Makefile`:

```makefile
.PHONY: test test-unit test-integration test-wal test-e2e smoke lint

# Run all fast tests
test: test-unit test-integration

# Unit tests only (no DB, no network)
test-unit:
	go test ./internal/... -race -short -count=1

# Integration tests with in-memory SQLite
test-integration:
	DATABASE_URL="file::memory:?cache=shared" \
	JWT_SECRET="test-secret" \
	go test ./internal/api/... -race -count=1 -tags=integration

# Integration tests with WAL mode (catches concurrency bugs)
test-wal:
	@mkdir -p /tmp/qcs-test-wal
	DATABASE_URL="/tmp/qcs-test-wal/test.db?_journal_mode=WAL" \
	JWT_SECRET="test-secret" \
	go test ./internal/api/... -race -count=1 -tags=integration,wal -timeout 120s
	@rm -rf /tmp/qcs-test-wal

# E2E tests
test-e2e:
	cd e2e && npx playwright test

# Smoke test
smoke:
	bash scripts/smoke-test.sh

# Linting
lint:
	golangci-lint run ./...

# Run everything (pre-push verification)
ci: lint test smoke
```

---

## 2. Test Data Seeding

### 2.1 Purpose

Every integration and E2E test needs a consistent starting state. Instead of each test creating its own ad-hoc data, use a centralized seeding package. This is critical for the forwarding workflow because the data chain is long: user → suite code → locker packages (various states) → ship requests (various states) → service requests → storage fees.

### 2.2 Implementation: `internal/testdata/seed.go`

```go
// Package testdata provides deterministic test fixtures for integration
// and E2E tests. All functions accept a *sql.DB and return created entities.
//
// Usage:
//   db := testdb.New(t) // in-memory SQLite
//   testdata.SeedAll(db) // full lifecycle data
//   // ... run tests against seeded data ...
//
// Or seed specific scenarios:
//   user := testdata.SeedCustomer(db)
//   pkg := testdata.SeedLockerPackage(db, user.ID, testdata.PackageStored)

package testdata

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/qcs-cargo/app/internal/models"
)

// ── Deterministic IDs (use these in assertions) ──

const (
	// Users
	CustomerAliceID   = "usr_alice_00000001"
	CustomerBobID     = "usr_bob_000000002"
	StaffWarehouseID  = "usr_staff_0000001"
	AdminID           = "usr_admin_0000001"

	// Alice's suite code
	AliceSuiteCode = "QCS-A1B2C3"
	BobSuiteCode   = "QCS-D4E5F6"

	// Alice's locker packages
	PkgAliceStored1   = "lpkg_alice_stor01" // Stored, day 5, from Amazon
	PkgAliceStored2   = "lpkg_alice_stor02" // Stored, day 25, expiring soon
	PkgAliceStored3   = "lpkg_alice_stor03" // Stored, day 32, storage fees accruing
	PkgAliceShipped   = "lpkg_alice_ship01" // Already shipped
	PkgAliceService   = "lpkg_alice_svc001" // Has pending photo request
	PkgAliceExpired   = "lpkg_alice_exp001" // Past disposal threshold

	// Alice's ship requests
	ShipReqAliceDraft = "sreq_alice_draft1"
	ShipReqAlicePaid  = "sreq_alice_paid01"
	ShipReqAliceShipped = "sreq_alice_ship01"

	// Bob's locker packages (for multi-customer tests)
	PkgBobStored1 = "lpkg_bob_stored01"

	// Unmatched package
	UnmatchedPkg1 = "unmatch_00000001"

	// Booking (drop-off flow)
	BookingAlice1 = "bkg_alice_00001"

	// Recipients
	RecipientGeorgetown = "rcpt_georgetown01"
	RecipientKingston   = "rcpt_kingston001"
)

// ── Seed Functions ──

// SeedAll creates a complete test world with data covering every major state.
// Call this once at the start of an integration test suite.
func SeedAll(db *sql.DB) error {
	seeders := []func(*sql.DB) error{
		SeedUsers,
		SeedRecipients,
		SeedLockerPackagesAlice,
		SeedLockerPackagesBob,
		SeedServiceRequests,
		SeedStorageFees,
		SeedShipRequests,
		SeedBookings,
		SeedUnmatchedPackages,
		SeedInboundTracking,
	}
	for _, s := range seeders {
		if err := s(db); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}
	return nil
}

// SeedUsers creates 4 users: 2 customers, 1 staff, 1 admin.
func SeedUsers(db *sql.DB) error {
	users := []struct {
		id, name, email, suiteCode, role string
	}{
		{CustomerAliceID, "Alice Johnson", "alice@test.com", AliceSuiteCode, "customer"},
		{CustomerBobID, "Bob Williams", "bob@test.com", BobSuiteCode, "customer"},
		{StaffWarehouseID, "Warehouse Staff", "staff@qcs-cargo.com", "", "staff"},
		{AdminID, "Admin User", "admin@qcs-cargo.com", "", "admin"},
	}
	for _, u := range users {
		_, err := db.Exec(`
			INSERT INTO users (id, name, email, suite_code, role, email_verified, status, free_storage_days, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, 'active', 30, datetime('now'), datetime('now'))
		`, u.id, u.name, u.email, u.suiteCode, u.role)
		if err != nil {
			return fmt.Errorf("seed user %s: %w", u.name, err)
		}
	}
	return nil
}

// SeedLockerPackagesAlice creates packages in every lifecycle state.
func SeedLockerPackagesAlice(db *sql.DB) error {
	now := time.Now()
	packages := []struct {
		id, sender, status string
		arrivedDaysAgo     int
		weight             float64
	}{
		{PkgAliceStored1, "Amazon", "stored", 5, 2.5},
		{PkgAliceStored2, "Walmart", "stored", 25, 4.0},      // Expiring soon
		{PkgAliceStored3, "eBay", "stored", 32, 1.8},          // Past free period
		{PkgAliceShipped, "Nike", "shipped", 15, 3.2},
		{PkgAliceService, "Best Buy", "service_pending", 10, 6.0},
		{PkgAliceExpired, "Target", "expired", 65, 2.0},
	}
	for _, p := range packages {
		arrived := now.AddDate(0, 0, -p.arrivedDaysAgo)
		freeExpires := arrived.AddDate(0, 0, 30)
		_, err := db.Exec(`
			INSERT INTO locker_packages
				(id, user_id, suite_code, sender_name, weight_lbs, condition, status,
				 storage_bay, arrived_at, free_storage_expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'good', ?, 'A1', ?, ?, ?, ?)
		`, p.id, CustomerAliceID, AliceSuiteCode, p.sender, p.weight, p.status,
			arrived.Format(time.RFC3339), freeExpires.Format(time.RFC3339),
			arrived.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("seed locker package %s: %w", p.id, err)
		}
	}
	return nil
}

// SeedRecipients, SeedLockerPackagesBob, SeedServiceRequests,
// SeedStorageFees, SeedShipRequests, SeedBookings,
// SeedUnmatchedPackages, SeedInboundTracking follow the same pattern.
// Each creates deterministic records with known IDs for assertion.
//
// IMPORTANT: Implement all of these following the pattern above.
// Every entity must have a const ID defined at the top of this file
// so tests can reference them without magic strings.

func SeedRecipients(db *sql.DB) error {
	// Create 2 recipients: Georgetown (Guyana), Kingston (Jamaica)
	// Use RecipientGeorgetown and RecipientKingston IDs
	// ... implement
	return nil
}

func SeedLockerPackagesBob(db *sql.DB) error {
	// Create 1 stored package for Bob (multi-customer isolation tests)
	// Use PkgBobStored1 ID
	// ... implement
	return nil
}

func SeedServiceRequests(db *sql.DB) error {
	// Create pending photo request for PkgAliceService
	// ... implement
	return nil
}

func SeedStorageFees(db *sql.DB) error {
	// Create 2 days of storage fees for PkgAliceStored3 (past free period)
	// ... implement
	return nil
}

func SeedShipRequests(db *sql.DB) error {
	// Create 3 ship requests in different states:
	// ShipReqAliceDraft: draft, references PkgAliceStored1
	// ShipReqAlicePaid: paid, references PkgAliceStored2
	// ShipReqAliceShipped: shipped, with tracking number
	// ... implement
	return nil
}

func SeedBookings(db *sql.DB) error {
	// Create 1 confirmed booking for Alice (drop-off flow)
	// Use BookingAlice1 ID
	// ... implement
	return nil
}

func SeedUnmatchedPackages(db *sql.DB) error {
	// Create 1 unmatched package pending resolution
	// Use UnmatchedPkg1 ID
	// ... implement
	return nil
}

func SeedInboundTracking(db *sql.DB) error {
	// Create 2 inbound tracking entries for Alice:
	// One "in_transit", one "delivered" (matched to PkgAliceStored1)
	// ... implement
	return nil
}
```

### 2.3 Test Database Helper: `internal/testdata/testdb.go`

```go
// Package testdata provides a test database helper that creates
// a fresh in-memory SQLite database with all migrations applied.
package testdata

import (
	"database/sql"
	"testing"

	"github.com/qcs-cargo/app/internal/db"
)

// NewTestDB creates a fresh in-memory SQLite database with all
// migrations applied. The database is closed when the test ends.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	conn, err := db.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	t.Cleanup(func() { conn.Close() })
	return conn
}

// NewSeededDB creates a test database with all seed data loaded.
func NewSeededDB(t *testing.T) *sql.DB {
	t.Helper()
	conn := NewTestDB(t)
	if err := SeedAll(conn); err != nil {
		t.Fatalf("failed to seed test db: %v", err)
	}
	return conn
}
```

### 2.4 Usage in Tests

```go
func TestLockerList_ReturnsOnlyStoredPackages(t *testing.T) {
	db := testdata.NewSeededDB(t)
	app := setupTestApp(db) // creates Fiber app with test config

	// Alice has 3 stored, 1 shipped, 1 service_pending, 1 expired
	req := httptest.NewRequest("GET", "/api/v1/locker?status=stored", nil)
	req.Header.Set("Authorization", "Bearer "+tokenForUser(testdata.CustomerAliceID))

	resp, _ := app.Test(req)
	assert.Equal(t, 200, resp.StatusCode)

	var body struct {
		Data       []models.LockerPackage `json:"data"`
		Pagination models.Pagination      `json:"pagination"`
	}
	json.NewDecoder(resp.Body).Decode(&body)

	assert.Equal(t, 3, len(body.Data))
	// Verify Alice cannot see Bob's packages
	for _, pkg := range body.Data {
		assert.Equal(t, testdata.CustomerAliceID, pkg.UserID)
	}
}
```

---

## 3. Unit Testing Strategy

### 3.1 What to Unit Test

Unit tests cover pure business logic with no database, no HTTP, no browser dependencies. These run with `go test -short` and execute in milliseconds.

| Package | Functions to Test | Priority |
|---------|------------------|----------|
| `internal/calc` | DimensionalWeight(), BillableWeight(), ShippingCost(), ConsolidationSavings(), VolumDiscount(), StorageFee() | **Critical** |
| `internal/validation` | ValidateEmail(), ValidatePhone(), ValidateCustomsDeclaration(), ValidateWeight(), ValidateSuiteCode() | **Critical** |
| `internal/models` | Status transition methods: Booking.CanCancel(), ShipRequest.CanModify(), LockerPackage.IsExpiring() | **High** |
| `internal/services/auth` | GenerateSuiteCode(), HashToken(), ValidatePasswordStrength() | **High** |
| `internal/services/storage` | CalculateDaysStored(), IsInFreePeriod(), DailyFeeAmount(), ShouldWarn(), ShouldDispose() | **Critical** |

### 3.2 Example: Pricing Calculation Tests

Create `internal/calc/pricing_test.go`:

```go
package calc_test

import (
	"testing"

	"github.com/qcs-cargo/app/internal/calc"
	"github.com/stretchr/testify/assert"
)

func TestDimensionalWeight(t *testing.T) {
	tests := []struct {
		name   string
		l, w, h float64
		want   float64
	}{
		{"standard box", 12, 12, 12, 10.41}, // 1728/166 = 10.41
		{"flat envelope", 15, 12, 1, 1.08},   // 180/166 = 1.08
		{"zero dimension", 0, 12, 12, 0},
		{"large box", 24, 24, 24, 83.28},     // 13824/166
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.DimensionalWeight(tt.l, tt.w, tt.h)
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestShippingCost(t *testing.T) {
	tests := []struct {
		name        string
		input       calc.ShippingInput
		wantTotal   float64
		wantSavings float64
	}{
		{
			name: "standard 5lb to Guyana",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 5.0,
				DeclaredValue: 100,
				Insurance:    false,
			},
			wantTotal:   17.50, // 5 * 3.50
			wantSavings: 0,
		},
		{
			name: "express 5lb to Guyana",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "express",
				ActualWeight: 5.0,
			},
			wantTotal: 21.88, // 5 * 3.50 * 1.25
		},
		{
			name: "door-to-door 5lb to Guyana",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "door_to_door",
				ActualWeight: 5.0,
			},
			wantTotal: 42.50, // (5 * 3.50) + 25
		},
		{
			name: "dimensional weight exceeds actual",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 5.0,
				Length: 24, Width: 24, Height: 24, // dim weight = 83.28
			},
			wantTotal: 291.48, // 83.28 * 3.50
		},
		{
			name: "volume discount 150lbs",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 150.0,
			},
			wantTotal: 498.75, // 150 * 3.50 * 0.95 (5% discount)
		},
		{
			name: "minimum charge",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 0.5,
			},
			wantTotal: 10.00, // min charge, not 0.5 * 3.50 = 1.75
		},
		{
			name: "insurance",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 5.0,
				DeclaredValue: 500,
				Insurance:    true,
			},
			wantTotal: 22.50, // (5 * 3.50) + (500 / 100)
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.CalculateShipping(tt.input)
			assert.InDelta(t, tt.wantTotal, result.Total, 0.01)
		})
	}
}

func TestConsolidationSavings(t *testing.T) {
	// Shipping 3 packages individually vs consolidated
	individual := []calc.ShippingInput{
		{Destination: "guyana", Service: "standard", ActualWeight: 2.0},
		{Destination: "guyana", Service: "standard", ActualWeight: 3.0},
		{Destination: "guyana", Service: "standard", ActualWeight: 1.5},
	}
	// Consolidated weight is typically less due to repackaging
	consolidated := calc.ShippingInput{
		Destination:  "guyana",
		Service:      "standard",
		ActualWeight: 5.5, // Less than 2+3+1.5=6.5 due to repackaging
	}

	savings := calc.ConsolidationSavings(individual, consolidated)
	assert.True(t, savings > 0, "consolidation should save money")
}
```

### 3.3 Example: Storage Fee Tests

Create `internal/services/storage/storage_test.go`:

```go
package storage_test

import (
	"testing"
	"time"

	"github.com/qcs-cargo/app/internal/services/storage"
	"github.com/stretchr/testify/assert"
)

func TestCalculateDaysStored(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		arrivedAt time.Time
		want      int
	}{
		{"just arrived", now, 0},
		{"5 days ago", now.AddDate(0, 0, -5), 5},
		{"30 days ago", now.AddDate(0, 0, -30), 30},
		{"60 days ago", now.AddDate(0, 0, -60), 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storage.CalculateDaysStored(tt.arrivedAt, now)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldChargeStorageFee(t *testing.T) {
	freeDays := 30
	tests := []struct {
		name    string
		day     int
		charge  bool
		amount  float64
	}{
		{"day 1 - free", 1, false, 0},
		{"day 29 - free", 29, false, 0},
		{"day 30 - last free day", 30, false, 0},
		{"day 31 - first charge", 31, true, 1.50},
		{"day 45 - charging", 45, true, 1.50},
		{"day 60 - still charging", 60, true, 1.50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charge, amount := storage.ShouldCharge(tt.day, freeDays, 1.50)
			assert.Equal(t, tt.charge, charge)
			assert.InDelta(t, tt.amount, amount, 0.001)
		})
	}
}

func TestShouldWarn(t *testing.T) {
	tests := []struct {
		name    string
		day     int
		free    int
		warn5   bool
		warn1   bool
		final   bool
		dispose bool
	}{
		{"day 10", 10, 30, false, false, false, false},
		{"day 25 - 5 day warning", 25, 30, true, false, false, false},
		{"day 29 - 1 day warning", 29, 30, false, true, false, false},
		{"day 55 - final notice", 55, 30, false, false, true, false},
		{"day 60 - dispose", 60, 30, false, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.warn5, storage.ShouldWarn5Day(tt.day, tt.free))
			assert.Equal(t, tt.warn1, storage.ShouldWarn1Day(tt.day, tt.free))
			assert.Equal(t, tt.final, storage.ShouldFinalNotice(tt.day))
			assert.Equal(t, tt.dispose, storage.ShouldDispose(tt.day))
		})
	}
}
```

---

## 4. Integration Testing

**Run integration tests** (in-memory SQLite, no external services):

```bash
go test ./internal/api/... -tags=integration -count=1
```

Optional env for consistency with CI:

```bash
DATABASE_URL=file::memory:?cache=shared JWT_SECRET=test-secret go test ./internal/api/... -tags=integration -count=1
```

Tests use `NewSeededDB(t)` and a test app with all routes; Stripe/Resend use fake keys from test config.

### 4.1 Setup: Test App Factory

Create `internal/api/testutil_test.go`:

```go
//go:build integration

package api_test

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/qcs-cargo/app/internal/api"
	"github.com/qcs-cargo/app/internal/testdata"
)

const testJWTSecret = "test-secret-key-for-integration-tests"

// setupTestApp creates a Fiber app with all routes registered,
// backed by a seeded in-memory database.
func setupTestApp(t *testing.T) *fiber.App {
	t.Helper()
	db := testdata.NewSeededDB(t)
	app := api.NewApp(api.Config{
		DB:              db,
		JWTSecret:       testJWTSecret,
		StripeKey:       "sk_test_fake",
		StripeWebhookSecret: "whsec_test_fake",
		ResendKey:       "re_test_fake",
		FromEmail:       "test@qcs-cargo.com",
		AppURL:          "http://localhost:3000",
	})
	return app
}

// tokenForUser generates a valid JWT for a seeded user.
func tokenForUser(userID, role string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
	})
	str, _ := token.SignedString([]byte(testJWTSecret))
	return str
}

func customerToken() string {
	return tokenForUser(testdata.CustomerAliceID, "customer")
}

func adminToken() string {
	return tokenForUser(testdata.AdminID, "admin")
}

func staffToken() string {
	return tokenForUser(testdata.StaffWarehouseID, "staff")
}
```

### 4.2 Locker API Tests

Create `internal/api/locker_test.go`:

```go
//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qcs-cargo/app/internal/models"
	"github.com/qcs-cargo/app/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockerList(t *testing.T) {
	app := setupTestApp(t)

	t.Run("returns only customer's stored packages", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/locker?status=stored", nil)
		req.Header.Set("Authorization", "Bearer "+customerToken())

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Data []models.LockerPackage `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&body)

		// Alice has 3 stored packages (PkgAliceStored1, 2, 3)
		assert.Equal(t, 3, len(body.Data))

		// Verify no cross-customer leakage
		for _, pkg := range body.Data {
			assert.Equal(t, testdata.CustomerAliceID, pkg.UserID)
		}
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/locker", nil)
		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("customer cannot see another customer's packages", func(t *testing.T) {
		bobToken := tokenForUser(testdata.CustomerBobID, "customer")
		req := httptest.NewRequest("GET", "/api/v1/locker", nil)
		req.Header.Set("Authorization", "Bearer "+bobToken)

		resp, _ := app.Test(req)
		var body struct {
			Data []models.LockerPackage `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&body)

		// Bob should only see his 1 package
		assert.Equal(t, 1, len(body.Data))
		assert.Equal(t, testdata.PkgBobStored1, body.Data[0].ID)
	})
}

func TestLockerSummary(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/locker/summary", nil)
	req.Header.Set("Authorization", "Bearer "+customerToken())

	resp, _ := app.Test(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		TotalStored     int `json:"total_stored"`
		PendingServices int `json:"pending_services"`
	}
	json.NewDecoder(resp.Body).Decode(&body)

	assert.Equal(t, 3, body.TotalStored)
	assert.Equal(t, 1, body.PendingServices) // PkgAliceService
}

func TestLockerDetail(t *testing.T) {
	app := setupTestApp(t)

	t.Run("returns package with photos", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/locker/"+testdata.PkgAliceStored1, nil)
		req.Header.Set("Authorization", "Bearer "+customerToken())

		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("cannot access other customer's package", func(t *testing.T) {
		bobToken := tokenForUser(testdata.CustomerBobID, "customer")
		req := httptest.NewRequest("GET", "/api/v1/locker/"+testdata.PkgAliceStored1, nil)
		req.Header.Set("Authorization", "Bearer "+bobToken)

		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
```

### 4.3 Ship Request API Tests

Create `internal/api/ship_request_test.go`:

```go
//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qcs-cargo/app/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateShipRequest(t *testing.T) {
	app := setupTestApp(t)

	t.Run("creates ship request with stored packages", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"package_ids":  []string{testdata.PkgAliceStored1, testdata.PkgAliceStored2},
			"destination_id": "guyana",
			"recipient_id": testdata.RecipientGeorgetown,
			"service_type": "standard",
			"consolidate":  true,
		})
		req := httptest.NewRequest("POST", "/api/v1/ship-requests", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+customerToken())
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, "draft", result["status"])
		assert.NotEmpty(t, result["confirmation_code"])
	})

	t.Run("rejects empty package list", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"package_ids":    []string{},
			"destination_id": "guyana",
			"service_type":   "standard",
		})
		req := httptest.NewRequest("POST", "/api/v1/ship-requests", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+customerToken())
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("rejects packages from another customer", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"package_ids":    []string{testdata.PkgBobStored1}, // Bob's package
			"destination_id": "guyana",
			"service_type":   "standard",
		})
		req := httptest.NewRequest("POST", "/api/v1/ship-requests", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+customerToken()) // Alice's token
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("rejects already-shipped packages", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"package_ids":    []string{testdata.PkgAliceShipped},
			"destination_id": "guyana",
			"service_type":   "standard",
		})
		req := httptest.NewRequest("POST", "/api/v1/ship-requests", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+customerToken())
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
```

### 4.4 RBAC Tests

Create `internal/api/rbac_test.go`:

```go
//go:build integration

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRBAC(t *testing.T) {
	app := setupTestApp(t)

	tests := []struct {
		name           string
		method, path   string
		token          string
		expectedStatus int
	}{
		// Customer cannot access admin routes
		{"customer to admin dashboard", "GET", "/api/v1/admin/dashboard", customerToken(), 403},
		{"customer to admin users", "GET", "/api/v1/admin/users", customerToken(), 403},
		{"customer to warehouse stats", "GET", "/api/v1/warehouse/stats", customerToken(), 403},

		// Staff can access warehouse but not admin
		{"staff to warehouse stats", "GET", "/api/v1/warehouse/stats", staffToken(), 200},
		{"staff to admin dashboard", "GET", "/api/v1/admin/dashboard", staffToken(), 403},

		// Admin can access everything
		{"admin to admin dashboard", "GET", "/api/v1/admin/dashboard", adminToken(), 200},
		{"admin to warehouse stats", "GET", "/api/v1/warehouse/stats", adminToken(), 200},

		// Unauthenticated cannot access protected routes
		{"no token to locker", "GET", "/api/v1/locker", "", 401},
		{"no token to admin", "GET", "/api/v1/admin/dashboard", "", 401},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			resp, _ := app.Test(req)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}
```

---

## 5. Offline Warehouse E2E Testing

### 5.1 Playwright Configuration

Create `e2e/playwright.config.ts`:

```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  use: {
    baseURL: 'http://localhost:8080',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
});
```

### 5.2 Offline Receiving Test

Create `e2e/tests/warehouse-offline.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';

test.describe('Warehouse Offline Receiving', () => {
  test.beforeEach(async ({ page }) => {
    // Login as warehouse staff
    // (implement helper that gets a magic link token from test API)
    await page.goto('/warehouse/locker-receive');
    await expect(page.locator('h1')).toContainText('Receive');
  });

  test('receives package while offline, syncs on reconnect', async ({ page, context }) => {
    // Step 1: Verify online indicator shows green
    await expect(page.locator('[data-testid="sync-status"]')).toHaveAttribute(
      'data-status', 'synced'
    );

    // Step 2: Go offline
    await context.setOffline(true);

    // Verify offline indicator appears
    await expect(page.locator('[data-testid="sync-status"]')).toHaveAttribute(
      'data-status', 'offline'
    );

    // Step 3: Scan a suite code
    await page.fill('[data-testid="suite-code-input"]', 'QCS-A1B2C3');
    await page.click('[data-testid="lookup-button"]');

    // Should find cached customer data
    await expect(page.locator('[data-testid="customer-name"]')).toContainText('Alice');

    // Step 4: Fill receiving form
    await page.fill('[data-testid="weight-input"]', '3.5');
    await page.selectOption('[data-testid="condition-select"]', 'good');
    await page.fill('[data-testid="sender-input"]', 'Amazon');

    // Step 5: Take photo (simulate with file upload)
    const fileChooserPromise = page.waitForEvent('filechooser');
    await page.click('[data-testid="photo-button"]');
    const fileChooser = await fileChooserPromise;
    await fileChooser.setFiles('e2e/fixtures/test-photo.jpg');

    // Step 6: Submit
    await page.click('[data-testid="receive-button"]');

    // Should show success (queued)
    await expect(page.locator('[data-testid="success-message"]')).toBeVisible();

    // Sync indicator should show 1 pending
    await expect(page.locator('[data-testid="sync-status"]')).toHaveAttribute(
      'data-status', 'pending'
    );
    await expect(page.locator('[data-testid="sync-count"]')).toContainText('1');

    // Step 7: Go back online
    await context.setOffline(false);

    // Wait for sync to complete
    await expect(page.locator('[data-testid="sync-status"]')).toHaveAttribute(
      'data-status', 'synced',
      { timeout: 10_000 }
    );

    // Step 8: Verify data persisted via API
    const response = await page.request.get('/api/v1/warehouse/packages?user_id=usr_alice_00000001&sort=created_at&order=desc');
    const body = await response.json();
    expect(body.data[0].sender_name).toBe('Amazon');
    expect(body.data[0].weight_lbs).toBe(3.5);
  });

  test('handles photo sync failure gracefully', async ({ page, context }) => {
    await context.setOffline(true);

    // Fill and submit a package (same as above, abbreviated)
    await page.fill('[data-testid="suite-code-input"]', 'QCS-A1B2C3');
    await page.click('[data-testid="lookup-button"]');
    await page.fill('[data-testid="weight-input"]', '2.0');
    await page.selectOption('[data-testid="condition-select"]', 'good');
    await page.fill('[data-testid="sender-input"]', 'Walmart');

    // Add photo
    const fc = page.waitForEvent('filechooser');
    await page.click('[data-testid="photo-button"]');
    (await fc).setFiles('e2e/fixtures/test-photo.jpg');

    await page.click('[data-testid="receive-button"]');
    await expect(page.locator('[data-testid="success-message"]')).toBeVisible();

    // Go online but intercept photo uploads to fail
    await context.setOffline(false);
    await page.route('**/api/v1/warehouse/packages/*/photos', (route) => {
      route.fulfill({ status: 500, body: '{"error":{"code":"UPLOAD_FAILED"}}' });
    });

    // Wait for text sync to succeed
    await page.waitForTimeout(3000);

    // Text data should be synced, photo should show as failed
    // Package exists on server but photo is pending
    await expect(page.locator('[data-testid="sync-status"]')).toHaveAttribute(
      'data-status', 'partial'
    );
  });

  test('handles sync conflict when suite code changed while offline', async ({ page, context }) => {
    await context.setOffline(true);

    // Receive package for Alice while offline
    await page.fill('[data-testid="suite-code-input"]', 'QCS-A1B2C3');
    await page.click('[data-testid="lookup-button"]');
    await page.fill('[data-testid="weight-input"]', '1.0');
    await page.selectOption('[data-testid="condition-select"]', 'good');
    await page.fill('[data-testid="sender-input"]', 'eBay');
    await page.click('[data-testid="receive-button"]');

    // While still offline, simulate admin changing the suite code via direct API
    // (This would happen in reality if admin reassigned the suite code)
    // We simulate by intercepting the sync POST to return 409 Conflict
    await page.route('**/api/v1/warehouse/locker-receive', (route) => {
      route.fulfill({
        status: 409,
        body: JSON.stringify({
          error: {
            code: 'SUITE_CODE_CONFLICT',
            message: 'Suite code QCS-A1B2C3 has been reassigned',
          }
        }),
      });
    });

    await context.setOffline(false);

    // Sync should surface the conflict
    await expect(page.locator('[data-testid="sync-status"]')).toHaveAttribute(
      'data-status', 'error',
      { timeout: 10_000 }
    );

    // Failed item should be visible for manual resolution
    await page.click('[data-testid="sync-status"]');
    await expect(page.locator('[data-testid="sync-error-item"]')).toContainText('QCS-A1B2C3');
    await expect(page.locator('[data-testid="sync-error-item"]')).toContainText('reassigned');
  });
});
```

---

## 6. Stripe Payment Testing

### 6.1 Stripe Mock Service

For integration tests, create a mock Stripe handler. Do NOT call real Stripe APIs in tests.

Create `internal/testdata/stripe_mock.go`:

```go
package testdata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// NewStripeMock creates an HTTP server that mimics Stripe's API.
// Pass its URL as the Stripe base URL in test configuration.
func NewStripeMock() *httptest.Server {
	mux := http.NewServeMux()

	// POST /v1/payment_intents
	mux.HandleFunc("/v1/payment_intents", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            "pi_test_" + r.FormValue("metadata[ship_request_id]"),
			"client_secret": "pi_test_secret_fake",
			"status":        "requires_payment_method",
			"amount":        r.FormValue("amount"),
			"currency":      "usd",
		})
	})

	return httptest.NewServer(mux)
}
```

### 6.2 Webhook Integration Tests

Create `internal/api/payment_webhook_test.go`:

```go
//go:build integration

package api_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qcs-cargo/app/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWebhookSecret = "whsec_test_fake"

// signStripeWebhook creates a valid Stripe webhook signature for testing.
func signStripeWebhook(payload []byte, secret string) string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	signedPayload := fmt.Sprintf("%s.%s", ts, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,v1=%s", ts, sig)
}

func TestWebhook_PaymentIntentSucceeded(t *testing.T) {
	app := setupTestApp(t)

	t.Run("marks ship request as paid and updates packages", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"id":   "evt_test_001",
			"type": "payment_intent.succeeded",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id":     "pi_test_001",
					"status": "succeeded",
					"metadata": map[string]interface{}{
						"ship_request_id": testdata.ShipReqAliceDraft,
						"type":            "ship_request",
					},
				},
			},
		})

		req := httptest.NewRequest("POST", "/api/v1/payments/webhook", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Stripe-Signature", signStripeWebhook(payload, testWebhookSecret))

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify ship request status changed to "paid"
		// (query the DB directly or use admin API)
		detailReq := httptest.NewRequest("GET",
			"/api/v1/ship-requests/"+testdata.ShipReqAliceDraft, nil)
		detailReq.Header.Set("Authorization", "Bearer "+customerToken())
		detailResp, _ := app.Test(detailReq)

		var sr map[string]interface{}
		json.NewDecoder(detailResp.Body).Decode(&sr)
		assert.Equal(t, "paid", sr["status"])
		assert.Equal(t, "paid", sr["payment_status"])
	})

	t.Run("idempotent: duplicate webhook does not error", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"id":   "evt_test_001", // Same event ID
			"type": "payment_intent.succeeded",
			"data": map[string]interface{}{
				"object": map[string]interface{}{
					"id":       "pi_test_001",
					"status":   "succeeded",
					"metadata": map[string]interface{}{"ship_request_id": testdata.ShipReqAliceDraft, "type": "ship_request"},
				},
			},
		})

		req := httptest.NewRequest("POST", "/api/v1/payments/webhook", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Stripe-Signature", signStripeWebhook(payload, testWebhookSecret))

		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusOK, resp.StatusCode) // Should not error
	})

	t.Run("invalid signature returns 400", func(t *testing.T) {
		payload := []byte(`{"type":"payment_intent.succeeded"}`)
		req := httptest.NewRequest("POST", "/api/v1/payments/webhook", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Stripe-Signature", "t=123,v1=invalidsignature")

		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestWebhook_PaymentIntentFailed(t *testing.T) {
	app := setupTestApp(t)

	payload, _ := json.Marshal(map[string]interface{}{
		"id":   "evt_test_002",
		"type": "payment_intent.payment_failed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":     "pi_test_002",
				"status": "requires_payment_method",
				"metadata": map[string]interface{}{
					"ship_request_id": testdata.ShipReqAlicePaid,
					"type":            "ship_request",
				},
				"last_payment_error": map[string]interface{}{
					"message": "Your card was declined.",
				},
			},
		},
	})

	req := httptest.NewRequest("POST", "/api/v1/payments/webhook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signStripeWebhook(payload, testWebhookSecret))

	resp, _ := app.Test(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify ship request reverted to pending_payment
	detailReq := httptest.NewRequest("GET",
		"/api/v1/ship-requests/"+testdata.ShipReqAlicePaid, nil)
	detailReq.Header.Set("Authorization", "Bearer "+customerToken())
	detailResp, _ := app.Test(detailReq)

	var sr map[string]interface{}
	json.NewDecoder(detailResp.Body).Decode(&sr)
	assert.Equal(t, "pending_payment", sr["status"])
	assert.Equal(t, "payment_failed", sr["payment_status"])

	// Verify locker packages reverted to "stored" (not "ship_requested")
	// ... query packages and assert status
}
```

### 6.3 Stripe CLI for Local Development

Add to `scripts/stripe-local.sh`:

```bash
#!/usr/bin/env bash
# Forward Stripe webhooks to local dev server
# Requires: stripe CLI installed and authenticated

echo "Forwarding Stripe webhooks to localhost:8080..."
stripe listen --forward-to localhost:8080/api/v1/payments/webhook

# To test specific events:
# stripe trigger payment_intent.succeeded \
#   --add payment_intent:metadata.ship_request_id=YOUR_ID \
#   --add payment_intent:metadata.type=ship_request
#
# stripe trigger payment_intent.payment_failed \
#   --add payment_intent:metadata.ship_request_id=YOUR_ID
```

---

## 7. Storage Fee Cron Job Testing

### 7.1 Unit Tests for Time Logic

Covered in Section 3.3 above (`storage_test.go`).

### 7.2 Integration Test for Full Cron Run

Create `internal/jobs/storage_cron_test.go`:

```go
//go:build integration

package jobs_test

import (
	"testing"
	"time"

	"github.com/qcs-cargo/app/internal/jobs"
	"github.com/qcs-cargo/app/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessDailyStorage(t *testing.T) {
	db := testdata.NewSeededDB(t)

	// Seed data includes:
	// PkgAliceStored3: arrived 32 days ago (2 days past free period)
	// PkgAliceStored1: arrived 5 days ago (well within free period)
	// PkgAliceStored2: arrived 25 days ago (5 days until fees)

	notifier := &mockNotifier{}
	processor := jobs.NewStorageFeeProcessor(db, notifier, 30, 1.50, 60)

	err := processor.ProcessDaily()
	require.NoError(t, err)

	t.Run("charges fee for package past free period", func(t *testing.T) {
		var count int
		db.QueryRow(`
			SELECT COUNT(*) FROM storage_fees
			WHERE locker_package_id = ? AND fee_date = date('now')
		`, testdata.PkgAliceStored3).Scan(&count)
		assert.Equal(t, 1, count)
	})

	t.Run("does not charge package within free period", func(t *testing.T) {
		var count int
		db.QueryRow(`
			SELECT COUNT(*) FROM storage_fees WHERE locker_package_id = ?
		`, testdata.PkgAliceStored1).Scan(&count)
		assert.Equal(t, 0, count)
	})

	t.Run("sends 5-day warning for expiring package", func(t *testing.T) {
		// PkgAliceStored2 is at day 25 = 5 days before free period ends
		assert.Contains(t, notifier.sent, notification{
			userID:  testdata.CustomerAliceID,
			event:   "storage_warning_5day",
			pkgID:   testdata.PkgAliceStored2,
		})
	})

	t.Run("does not send warning for non-expiring package", func(t *testing.T) {
		for _, n := range notifier.sent {
			if n.pkgID == testdata.PkgAliceStored1 {
				t.Error("should not warn for package with 25 days remaining")
			}
		}
	})

	t.Run("idempotent: running twice does not double-charge", func(t *testing.T) {
		err := processor.ProcessDaily()
		require.NoError(t, err)

		var count int
		db.QueryRow(`
			SELECT COUNT(*) FROM storage_fees
			WHERE locker_package_id = ? AND fee_date = date('now')
		`, testdata.PkgAliceStored3).Scan(&count)
		assert.Equal(t, 1, count) // Still 1, not 2
	})
}

func TestStorageDisposal(t *testing.T) {
	db := testdata.NewSeededDB(t)

	// PkgAliceExpired is 65 days old (past 60-day disposal threshold)
	notifier := &mockNotifier{}
	processor := jobs.NewStorageFeeProcessor(db, notifier, 30, 1.50, 60)

	err := processor.ProcessDisposals()
	require.NoError(t, err)

	var status string
	db.QueryRow(`SELECT status FROM locker_packages WHERE id = ?`,
		testdata.PkgAliceExpired).Scan(&status)
	assert.Equal(t, "disposed", status)

	// Verify activity log entry
	var logCount int
	db.QueryRow(`
		SELECT COUNT(*) FROM activity_log
		WHERE resource_type = 'locker_package'
		AND resource_id = ?
		AND action = 'disposed'
	`, testdata.PkgAliceExpired).Scan(&logCount)
	assert.Equal(t, 1, logCount)
}

// ── Mock Notifier ──

type notification struct {
	userID, event, pkgID string
}

type mockNotifier struct {
	sent []notification
}

func (m *mockNotifier) Send(userID, event, pkgID string, _ map[string]string) error {
	m.sent = append(m.sent, notification{userID, event, pkgID})
	return nil
}
```

---

## 8. Load Testing

### 8.1 k6 Script

Create `loadtest/basic.js`:

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },   // Ramp up to 50 users
    { duration: '1m', target: 100 },   // Hold at 100 users
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'],   // p95 < 100ms (PRD requirement)
    http_req_failed: ['rate<0.01'],     // < 1% error rate
  },
};

const BASE = __ENV.BASE_URL || 'http://localhost:8080';

// Get a test auth token (implement a test-only endpoint or pre-generate)
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';

const headers = {
  'Authorization': `Bearer ${AUTH_TOKEN}`,
  'Content-Type': 'application/json',
};

export default function () {
  // Mix of endpoints weighted by expected real traffic

  // 40% - Locker list (most frequent customer action)
  if (Math.random() < 0.4) {
    const res = http.get(`${BASE}/api/v1/locker`, { headers });
    check(res, { 'locker 200': (r) => r.status === 200 });
  }

  // 20% - Ship request estimate (customer checking prices)
  if (Math.random() < 0.2) {
    const res = http.get(`${BASE}/api/v1/calculator?destination=guyana&service=standard&weight=5`, { headers });
    check(res, { 'calculator 200': (r) => r.status === 200 });
  }

  // 15% - Public tracking lookup
  if (Math.random() < 0.15) {
    const res = http.get(`${BASE}/api/v1/track/QCS-2026-001234`);
    check(res, { 'track responds': (r) => r.status === 200 || r.status === 404 });
  }

  // 10% - Warehouse receive (write operation)
  if (Math.random() < 0.1) {
    const payload = JSON.stringify({
      suite_code: 'QCS-A1B2C3',
      weight_lbs: 2.5 + Math.random() * 5,
      condition: 'good',
      sender_name: 'LoadTest Sender',
    });
    const res = http.post(`${BASE}/api/v1/warehouse/locker-receive`, payload, { headers });
    check(res, { 'receive 201': (r) => r.status === 201 });
  }

  // 10% - Admin dashboard
  if (Math.random() < 0.1) {
    const res = http.get(`${BASE}/api/v1/admin/dashboard`, { headers });
    check(res, { 'admin 200': (r) => r.status === 200 });
  }

  // 5% - Locker summary
  if (Math.random() < 0.05) {
    const res = http.get(`${BASE}/api/v1/locker/summary`, { headers });
    check(res, { 'summary 200': (r) => r.status === 200 });
  }

  sleep(0.5 + Math.random());
}
```

### 8.2 Makefile Target

```makefile
# Load test (requires k6 installed)
load-test:
	k6 run loadtest/basic.js --env BASE_URL=http://localhost:8080 --env AUTH_TOKEN=$(shell go run cmd/testtoken/main.go)
```

### 8.3 When to Run

- **Not in CI** (too slow, non-deterministic).
- Run manually before each phase completion.
- Run before production deployment.
- Run after switching from SQLite to PostgreSQL to compare.

---

## 9. go-app Component Testing

### 9.1 Architecture Rule

**All business logic must be extractable from components.** Components should be thin wrappers that call pure functions and render results.

```
BAD:  Component calculates dimensional weight inside Render()
GOOD: Component calls calc.DimensionalWeight() and renders the result

BAD:  Component validates customs form inside onChange handler
GOOD: Component calls validation.ValidateCustomsItem() and renders errors

BAD:  Component decides ship request state transitions in button handler
GOOD: Component calls shipRequest.CanSubmit() and enables/disables button
```

### 9.2 Extractable Logic Checklist

These functions MUST live outside `frontend/` in `internal/` packages so they are testable with standard `go test`:

| Function | Package | Used By |
|----------|---------|---------|
| `DimensionalWeight(l, w, h)` | `internal/calc` | Calculator page, Ship wizard step 4 |
| `BillableWeight(actual, dim)` | `internal/calc` | Calculator, Ship wizard |
| `CalculateShipping(input)` | `internal/calc` | Calculator, Ship wizard step 4 |
| `ConsolidationSavings(individual, consolidated)` | `internal/calc` | Ship wizard step 1 |
| `VolumeDiscount(weight)` | `internal/calc` | Ship wizard step 4 |
| `ValidateEmail(s)` | `internal/validation` | Login, Register, Contact |
| `ValidatePhone(s)` | `internal/validation` | Contact, Recipients |
| `ValidateCustomsItem(item)` | `internal/validation` | Ship wizard step 3 |
| `ValidateWeight(w)` | `internal/validation` | Calculator, Receiving |
| `StorageDaysRemaining(arrived, freeDays)` | `internal/calc` | Package inbox, Detail |
| `StorageBarColor(daysUsed, freeDays)` | `internal/calc` | Package inbox cards |
| `FormatSuiteAddress(name, suiteCode)` | `internal/format` | Mailbox page |
| `CanConsolidate(packages)` | `internal/validation` | Ship wizard step 1 |
| `ShipRequestCanModify(status)` | `internal/models` | Ship request detail |
| `BookingCanCancel(status, scheduledDate)` | `internal/models` | Booking detail |

### 9.3 What Playwright Tests Cover (Component Behavior)

Playwright covers what unit tests cannot: DOM rendering, user interaction, navigation, and browser API integration (camera, IndexedDB).

Key Playwright component tests:

- Package Inbox: selecting packages updates floating bar count and weight.
- Package Inbox: storage progress bar shows correct color at each threshold.
- Ship Wizard: cannot proceed from step 1 without selecting packages.
- Ship Wizard: step 3 customs form validates required fields inline.
- Ship Wizard: cost estimate updates when service type changes.
- Mailbox: copy button puts full address on clipboard.
- Inbound Tracking: adding tracking number shows "Tracking" status.

---

## 10. Test File Organization

```
qcs-cargo/
├── internal/
│   ├── api/
│   │   ├── locker.go
│   │   ├── locker_test.go          # //go:build integration
│   │   ├── ship_request.go
│   │   ├── ship_request_test.go    # //go:build integration
│   │   ├── payment_webhook.go
│   │   ├── payment_webhook_test.go # //go:build integration
│   │   ├── rbac_test.go            # //go:build integration
│   │   └── testutil_test.go        # shared test helpers
│   ├── calc/
│   │   ├── pricing.go
│   │   ├── pricing_test.go         # pure unit tests (no build tags)
│   │   ├── storage.go
│   │   └── storage_test.go
│   ├── validation/
│   │   ├── validation.go
│   │   └── validation_test.go      # pure unit tests
│   ├── services/
│   │   └── storage/
│   │       ├── storage.go
│   │       └── storage_test.go     # pure unit tests
│   ├── jobs/
│   │   ├── storage_cron.go
│   │   └── storage_cron_test.go    # //go:build integration
│   ├── models/
│   │   ├── models.go
│   │   └── models_test.go          # pure unit tests (state transitions)
│   └── testdata/
│       ├── seed.go                 # deterministic test fixtures
│       ├── testdb.go               # in-memory DB factory
│       └── stripe_mock.go          # mock Stripe server
├── e2e/
│   ├── playwright.config.ts
│   ├── fixtures/
│   │   └── test-photo.jpg
│   └── tests/
│       ├── forwarding-flow.spec.ts # Full forwarding lifecycle
│       ├── dropoff-flow.spec.ts    # Full booking lifecycle
│       ├── warehouse-offline.spec.ts
│       ├── admin-operations.spec.ts
│       └── auth.spec.ts
├── loadtest/
│   └── basic.js                    # k6 load test script
├── scripts/
│   ├── smoke-test.sh
│   ├── stripe-local.sh
│   └── seed-dev.sh                 # Seed dev database
├── .github/
│   └── workflows/
│       └── ci.yml
└── Makefile
```

---

## Summary: What to Build and When

| Phase | Testing Work |
|-------|-------------|
| **Phase 0** | Set up `testdata/` package, `testdb.go`, `Makefile` targets, `ci.yml`, `smoke-test.sh`. Write pricing unit tests. |
| **Phase 1** | Auth integration tests. Smoke test covers auth + public routes. RBAC tests. |
| **Phase 2** | Locker API tests. Ship request tests. Storage fee unit tests. Customs validation tests. |
| **Phase 3** | Admin endpoint tests. Webhook tests (success, failure, duplicate, invalid signature). |
| **Phase 4** | Playwright offline tests. Storage cron integration tests. Service queue tests. |
| **Phase 5** | E2E full flows (Playwright). Load testing with k6. Coverage audit. |
