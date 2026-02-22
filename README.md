# QCS Cargo

**Parcel Forwarding · Air Freight · Warehouse Operations**

Unified product per QCS Cargo Unified PRD v3.

- **Repo:** [github.com/Qcsinc23/qcs-cargo](https://github.com/Qcsinc23/qcs-cargo) Stack: Go Fiber API + SQLite/Postgres + WASM PWA frontend (go-app).

## Quick start

```bash
# Migrate database (creates qcs.db with WAL)
make migrate

# Run server (serves API + static app at :8080)
make run
```

- **Health:** [http://localhost:8080/api/v1/health](http://localhost:8080/api/v1/health)
- **App:** [http://localhost:8080](http://localhost:8080)

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
| `MIGRATIONS_DIR` | Migration directory for migrate binary (default: `sql/migrations`) |

## Commands

- `make build` — build server + migrate binaries  
- `make run` — build and run server  
- `make migrate` — run migrations  
- `make test` — run tests  
- `make wasm` — build frontend to `web/app.wasm` and copy `wasm_exec.js`  
- `make sqlc` — regenerate sqlc code from `sql/`  

## Phase 0 deliverable

- ✅ Go module, Fiber server, `/api/v1/health`
- ✅ SQLite + WAL, migrations for all PRD Section 5 tables
- ✅ sqlc setup and generated code
- ✅ go-app-style frontend skeleton (stdlib WASM)
- ✅ Shared models (User, LockerPackage, ShipRequest, Booking)
- ✅ CI (lint, test, build server, build WASM) and Dockerfile

Next: Phase 1 — Auth + public pages (magic link, suite code, all public routes).
