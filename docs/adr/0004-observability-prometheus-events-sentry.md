# ADR-0004: Observability Baseline (Prometheus + Events Table + Optional Sentry)

Status: Accepted  
Date: 2026-02-28

## Context

The service currently has three observability layers:

- Prometheus HTTP metrics via middleware and public `/metrics`.
- Application events persisted to `observability_events` (analytics, performance, error, business) through async best-effort writes.
- Optional Sentry exception capture when `SENTRY_DSN` is configured.

Admin insights endpoints aggregate `observability_events` for operational summaries.

## Decision

Use Prometheus for request/service telemetry, `observability_events` for application-level event analytics, and Sentry as an optional external error sink.

## Consequences

- Core monitoring works without external vendors.
- Admin reporting can be generated directly from first-party event data.
- Event persistence is intentionally best-effort and can drop events if the in-memory queue is saturated.
- Sentry adds deeper external error triage only when configured.
