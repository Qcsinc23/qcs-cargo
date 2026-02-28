# ADR-0002: SQLite-First Persistence with SQL Migrations

Status: Accepted  
Date: 2026-02-28

## Context

Current DB wiring uses the `modernc.org/sqlite` driver with SQLite pragmas (`journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout`). Default runtime DSN is `file:qcs.db?_journal_mode=WAL`.

Schema changes are managed by versioned SQL files in `sql/migrations`, executed by `cmd/migrate` and `internal/db/migrate.go` with migration tracking in `schema_migrations`. Both up and down directions are supported.

## Decision

Adopt SQLite as the primary operational datastore and require SQL-file migrations (up/down) for schema evolution.

## Consequences

- Local development and single-node production remain straightforward.
- Migration history is explicit, versioned, and reversible.
- Write concurrency and horizontal scale are constrained by SQLite characteristics.
- Releases must continue to run migrations as a first-class deployment step.
