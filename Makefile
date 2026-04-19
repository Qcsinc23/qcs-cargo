# QCS Cargo — PRD 13.1 build commands
.PHONY: build run migrate test test-unit test-integration test-e2e loadtest loadtest-auth ci lint deps smoke backup restore-check assets

deps:
	go mod download
	go mod tidy

build:
	go build -o qcs-server ./cmd/server
	go build -o qcs-migrate ./cmd/migrate

# Phase 2.1 (DEF-002 + DEF-004): compile Tailwind CSS locally so the runtime
# does not depend on cdn.tailwindcss.com or cdn.jsdelivr.net. Output lands at
# internal/static/css/tailwind.css and is shipped via the embedded FS.
# Requires Node + npm. The first run installs Tailwind 3.4.x into
# tools/tailwind/node_modules; subsequent runs reuse it.
assets:
	@if ! command -v npm >/dev/null 2>&1; then echo "npm is required to build static assets (Tailwind CSS)"; exit 1; fi
	@if [ ! -d tools/tailwind/node_modules ]; then \
		echo "[assets] installing Tailwind toolchain (first run)"; \
		(cd tools/tailwind && npm install --no-fund --no-audit); \
	fi
	@echo "[assets] compiling internal/static/css/tailwind.css"
	@(cd tools/tailwind && npm run --silent build)
	@echo "[assets] done: $$(wc -c < internal/static/css/tailwind.css) bytes"

run: build
	./qcs-server

migrate: build
	./qcs-migrate

test:
	go test ./cmd/... ./internal/... -race -cover

test-unit:
	go test ./internal/... -race -short -count=1

test-integration:
	DATABASE_URL=file::memory:?cache=shared JWT_SECRET=test go test ./internal/api/... -race -count=1 -tags=integration

test-e2e:
	cd e2e && npx playwright test

loadtest:
	@if ! command -v k6 >/dev/null 2>&1; then echo "k6 is not installed. Install k6 to run load tests."; exit 1; fi
	k6 run loadtest/basic.js --env BASE_URL=$${BASE_URL:-http://localhost:8080}

loadtest-auth:
	@if ! command -v k6 >/dev/null 2>&1; then echo "k6 is not installed. Install k6 to run load tests."; exit 1; fi
	k6 run loadtest/auth-rate-limit.js --env BASE_URL=$${BASE_URL:-http://localhost:8080}

ci: lint test-unit test-integration smoke

lint:
	golangci-lint run ./cmd/... ./internal/...

# Generate sqlc code (requires sqlc: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest)
sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate

# Smoke test: build, migrate test DB, start server, hit key endpoints
smoke:
	chmod +x scripts/smoke-test.sh
	./scripts/smoke-test.sh

# Verify Stripe config (app API + optional CLI). Start server first with STRIPE_SECRET_KEY set.
stripe-verify:
	chmod +x scripts/stripe-verify.sh
	./scripts/stripe-verify.sh

# Hot-backup the SQLite database using the official online .backup command.
# Honors DATABASE_PATH (default qcs.db) and BACKUP_DIR (default ./backups).
# The .backup command is safe to run while the server is up — SQLite copies
# pages atomically while holding only short shared locks.
backup:
	@DB_PATH="$${DATABASE_PATH:-qcs.db}"; \
	BACKUP_DIR="$${BACKUP_DIR:-./backups}"; \
	mkdir -p "$$BACKUP_DIR"; \
	STAMP="$$(date +%Y%m%dT%H%M%S)"; \
	OUT="$$BACKUP_DIR/qcs-$$STAMP.db"; \
	echo "[backup] $$DB_PATH -> $$OUT"; \
	sqlite3 "$$DB_PATH" ".backup '$$OUT'"; \
	sqlite3 "$$OUT" "PRAGMA integrity_check;" | head -1 | grep -q '^ok$$' || { echo "[backup] integrity check FAILED for $$OUT"; exit 1; }; \
	sha256sum "$$OUT" > "$$OUT.sha256" 2>/dev/null || shasum -a 256 "$$OUT" > "$$OUT.sha256"; \
	echo "[backup] checksum written to $$OUT.sha256"

# Verify a backup file is restorable. Usage: BACKUP=path/to/qcs-XYZ.db make restore-check
restore-check:
	@if [ -z "$$BACKUP" ]; then echo "Usage: BACKUP=path/to/qcs-XYZ.db make restore-check"; exit 2; fi; \
	if [ ! -f "$$BACKUP" ]; then echo "[restore-check] $$BACKUP not found"; exit 1; fi; \
	sqlite3 "$$BACKUP" "PRAGMA integrity_check;" | head -1 | grep -q '^ok$$' || { echo "[restore-check] integrity_check FAILED"; exit 1; }; \
	USERS=$$(sqlite3 "$$BACKUP" "SELECT COUNT(*) FROM users;" 2>/dev/null || echo "?"); \
	SHIP=$$(sqlite3 "$$BACKUP" "SELECT COUNT(*) FROM ship_requests;" 2>/dev/null || echo "?"); \
	echo "[restore-check] OK users=$$USERS ship_requests=$$SHIP"
