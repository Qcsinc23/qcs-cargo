# Architecture Decision Records (ADR)

This directory tracks accepted architecture decisions that are already implemented in this repository.

## Index

| ADR | Title | Status | Date |
| --- | --- | --- | --- |
| [0001](./0001-go-fiber-monolith.md) | Go Fiber monolith serving API, static, and WASM | Accepted | 2026-02-28 |
| [0002](./0002-sqlite-first-migrations.md) | SQLite-first persistence with SQL up/down migrations | Accepted | 2026-02-28 |
| [0003](./0003-jwt-access-session-refresh.md) | JWT access tokens with DB-backed refresh sessions | Accepted | 2026-02-28 |
| [0004](./0004-observability-prometheus-events-sentry.md) | Observability baseline: Prometheus + events table + optional Sentry | Accepted | 2026-02-28 |
| [0005](./0005-single-replica-until.md) | Single-replica until measured load crosses defined thresholds | Accepted | 2026-04-19 |

## ADR Format

Each ADR uses:

- Status
- Date
- Context
- Decision
- Consequences
