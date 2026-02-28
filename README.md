# QCS Cargo

**Parcel Forwarding · Air Freight · Warehouse Operations**

Unified product per QCS Cargo Unified PRD v3.

- **Repo:** [github.com/Qcsinc23/qcs-cargo](https://github.com/Qcsinc23/qcs-cargo) Stack: Go Fiber API + SQLite/Postgres + WASM PWA frontend (go-app).

## Audit remediation status

- Canonical remediation tracker: `findings_status.md`
- Current snapshot (2026-02-28): `IMPLEMENTED 83`, `OPEN 37`, `TOTAL 120`
- Implementation roadmap and tranche sequencing: `plans/IMPLEMENTATION_PLAN.md`
- Change history: [CHANGELOG.md](CHANGELOG.md)

## Changelog policy

- Keep `CHANGELOG.md` in Keep a Changelog format with date-based entries.
- Update `## [Unreleased]` during active work and move items to a dated section when a remediation wave is completed.
- Keep entries concise and focused on externally meaningful behavior, contracts, and operational changes.

## Architecture and API docs

- OpenAPI spec: [docs/api/openapi.yaml](docs/api/openapi.yaml)
- API docs usage and rendering: [docs/api/README.md](docs/api/README.md)
- Architecture decisions (ADR): [docs/adr/README.md](docs/adr/README.md)
- Database schema reference: [docs/database/SCHEMA.md](docs/database/SCHEMA.md)

## Quick start

```bash
# Migrate database (creates qcs.db with WAL)
make migrate

# Run server (serves API + static app at :8080)
make run
```

- **Health:** [http://localhost:8080/api/v1/health](http://localhost:8080/api/v1/health)
- **Metrics:** [http://localhost:8080/metrics](http://localhost:8080/metrics) (Prometheus scrape endpoint)
- **App:** [http://localhost:8080](http://localhost:8080)

**Dev seed:** `./scripts/seed-dev.sh` (requires `qcs-migrate` and `sqlite3`).

## Go Version Policy

- CI uses the Go version declared in `go.mod` (`go` directive) as the source of truth.
- The workflow contains a `go-version-policy` job that resolves `go.mod` version and validates toolchain setup before test jobs run.
- Keep local and CI toolchains aligned with `go.mod` to avoid drift.

## Build WASM frontend (optional)

```bash
make wasm
```

This copies `wasm_exec.js` from your Go install into `web/` and builds `web/app.wasm` from `./frontend`. Reload the app in the browser to run the WASM UI.

## Project layout (PRD 2.2)

| Path | Purpose |
|------|--------|
| `cmd/server` | Fiber API + static/WASM serving |
| `cmd/migrate` | Database migration runner |
| `internal/api` | Route handlers (health, auth, locker, …) |
| `internal/db` | DB connection, migrations, sqlc-generated queries |
| `internal/models` | Shared domain types |
| `internal/static` | Embedded index.html |
| `frontend` | WASM app (Go → JS/WASM) |
| `sql/migrations` | Schema migrations (run in order) |
| `sql/queries` | sqlc SQL queries |
| `sql/schema` | sqlc schema (users, etc.) |
| `web` | Static assets + WASM output |

## Environment

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | SQLite path or Postgres URL (default: `file:qcs.db?_journal_mode=WAL`) |
| `PORT` | Server port (default: 8080) |
| `APP_URL` | Base URL for magic links, cookies (production: `https://qcs-cargo.com`) |
| `MIGRATIONS_DIR` | Migration directory for migrate binary (default: `sql/migrations`) |
| `STRIPE_SECRET_KEY` | Stripe secret key (sk_live_/sk_test_) for PaymentIntents |
| `STRIPE_PUBLISHABLE_KEY` | Stripe publishable key (pk_live_/pk_test_) for pay page |
| `RESEND_API_KEY` | Resend API key for transactional email; also surfaced as a boolean config flag in admin system-health |

See `.env.example` for a full list.

### Observability endpoints and config

- `GET /metrics` exposes Prometheus metrics (text exposition format) for scraping.
- `GET /api/v1/admin/system-health` (admin-only) returns operational counters and observability pointers, including `metrics_endpoint`, `generated_at`, `stripe_configured`, and `resend_configured`.
- `GET /api/v1/admin/insights` (admin-only) returns analytics/performance/error/business summaries from `observability_events`. Query params: `window_days` (1-90), `slow_ms` (50-10000), `slow_limit` (1-20).
- Observability config flags in `system-health` derive from environment presence checks for `STRIPE_SECRET_KEY` and `RESEND_API_KEY` (secrets are not returned).

## Production (qcs-cargo.com)

- Set `APP_URL=https://qcs-cargo.com` and use HTTPS.
- In **Stripe Dashboard**: add `https://qcs-cargo.com` to allowed redirect/checkout domains if required.
- Verify Stripe: run `make stripe-verify` (with server running and keys set).

## Stripe CLI configuration

- A **.env** file (gitignored) can hold `STRIPE_SECRET_KEY`, `STRIPE_PUBLISHABLE_KEY`, and `STRIPE_WEBHOOK_SECRET`. The server loads it on startup via godotenv.
- To configure Stripe via CLI: `stripe config --list`, `stripe webhook_endpoints list --live`, or create a webhook with `stripe webhook_endpoints create --url=https://qcs-cargo.com/api/webhooks/stripe ... --live` (use a secret key; the create response includes the signing secret for `STRIPE_WEBHOOK_SECRET`).
- The Stripe CLI can use your live key: `stripe config --set live_mode_api_key sk_live_...` so `stripe balance retrieve --live` and other commands work.

## Testing & integrations

See **[docs/TESTING_AND_INTEGRATIONS.md](docs/TESTING_AND_INTEGRATIONS.md)** for the full testing and integration strategy. Use it when building and completing the application. It covers:

- CI pipeline (lint, unit, integration, smoke, E2E)
- Test data seeding (`internal/testdata/`)
- Unit tests (pricing, storage, validation)
- Integration tests (API with in-memory SQLite): `go test ./internal/api/... -tags=integration -count=1` (optionally `DATABASE_URL=file::memory:?cache=shared`)
- Integration coverage now includes auth (`/auth/magic-link/verify`, `/auth/logout` contract), bookings (`/bookings` create/list), and locker flows (`/locker` list and service-request validation)
- Stripe payment and webhook testing
- Storage fee cron job tests
- Playwright E2E and offline warehouse tests
- Load testing (k6)
- Test file organization and go-app component testing

Implement tests and CI steps according to that doc as features are added.

Dependency update automation is configured with Dependabot for:
- Go modules (`/`)
- npm dependencies in `e2e/`
- GitHub Actions workflows

### API contract notes

- `POST /api/v1/auth/logout` intentionally returns `204 No Content` with an empty response body. Clients should treat the status code as success and not expect JSON content.
- `GET /api/v1/destinations` and `GET /api/v1/destinations/:id` are DB-backed via the `destinations` catalog table. A static fallback list is used only if destination DB access fails unexpectedly.
- `GET /api/v1/locker` supports pagination query params: `limit` (default `20`, max `100`) and `page` (default `1`). Response includes `data`, `page`, `limit`, `total`, and `status`.
- `GET /api/v1/admin/system-health` (admin-only) returns monitoring snapshot data: status, DB health, Stripe/Resend config flags, `metrics_endpoint`, queue/count metrics, and `generated_at`.

**E2E (Playwright):** From the project root, run: `cd e2e && npm ci && npx playwright install chromium && npx playwright test`. Ensure the server is running at http://localhost:8080 (e.g. `make run` in another terminal). In local smoke mode, set `RESEND_API_KEY` empty so verification/magic-link email sends operate as no-op.

## Commands

- `make build` — build server + migrate binaries  
- `make run` — build and run server  
- `make migrate` — run migrations  
- `make test` — run full unit tests  
- `make test-unit` — unit tests only (`./internal/...`, no integration)  
- `make test-integration` — API integration tests (in-memory SQLite)  
- `make test-e2e` — Playwright E2E tests (`e2e/`)  
- `make loadtest` — k6 load test (`loadtest/basic.js`) against `http://localhost:8080`  
- `make loadtest-auth` — k6 auth burst/rate-limit scenario (`loadtest/auth-rate-limit.js`)  
- `make ci` — lint, test-unit, test-integration, smoke  
- `make smoke` — smoke test (build, migrate, start, curl health/destinations/auth)  
- `make stripe-verify` — verify Stripe config (app API + optional Stripe CLI)  
- `make wasm` — build frontend to `web/app.wasm` and copy `wasm_exec.js`  
- `make sqlc` — regenerate sqlc code from `sql/`  

## Admin console (Phase 3)

Admin routes live under `/api/v1/admin/` and the UI under `/admin`. Only users with role `admin` can access them; others receive 403.

**How to set a user as admin**

1. **Database update (SQLite):**  
   After the user exists (e.g. after sign-up or magic-link login), set their role in the DB:
   ```bash
   sqlite3 qcs.db "UPDATE users SET role = 'admin', updated_at = datetime('now') WHERE email = 'admin@example.com';"
   ```
2. **Seed script:**  
   You can add a seed or migration that inserts or updates a known admin user (e.g. by email) with `role = 'admin'`. The `users` table has a `role` column (default `customer`); valid values include `customer`, `staff`, `admin`.

After updating, log in as that user and open `/admin` to see the admin dashboard and lists (ship requests, locker packages, service queue, unmatched, bookings, users).

## PRD implementation status (baseline)

This phase checklist reflects baseline PRD delivery history. Audit remediation progress is tracked separately in `findings_status.md`.

- **Phase 0** — Module, Fiber, health, SQLite/WAL, migrations, sqlc, frontend skeleton, models, CI: ✅ done
- **Phase 1** — Auth + public pages (magic link, suite code, public routes): ✅ done
- **Phase 2** — Dashboard, forwarding, templates: ✅ done
- **Phase 3** — Admin, reports, settings, search, activity: ✅ done
- **Phase 4** — Warehouse, receiving, staging, manifests, exceptions: ✅ done
- **Phase 5** — Jobs, CORS, rate limit, E2E, accessibility, deployment docs: ✅ done

## Phase 0 deliverable

- ✅ Go module, Fiber server, `/api/v1/health`
- ✅ SQLite + WAL, migrations for all PRD Section 5 tables
- ✅ sqlc setup and generated code
- ✅ go-app-style frontend skeleton (stdlib WASM)
- ✅ Shared models (User, LockerPackage, ShipRequest, Booking)
- ✅ CI (lint, test, build server, build WASM) and Dockerfile

Next: Phase 1 — Auth + public pages (magic link, suite code, all public routes).
