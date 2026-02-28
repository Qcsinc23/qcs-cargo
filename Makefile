# QCS Cargo — PRD 13.1 build commands
.PHONY: build run migrate test test-unit test-integration test-e2e loadtest loadtest-auth ci lint wasm deps smoke

deps:
	go mod download
	go mod tidy

build:
	go build -o qcs-server ./cmd/server
	go build -o qcs-migrate ./cmd/migrate

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

# Copy Go's WASM loader, sync frontend images to web/images, and build frontend to web/
wasm:
	cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" web/
	@mkdir -p web/images && cp -R frontend/static/images/* web/images/ 2>/dev/null || true
	GOOS=js GOARCH=wasm go build -o web/app.wasm ./frontend

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
