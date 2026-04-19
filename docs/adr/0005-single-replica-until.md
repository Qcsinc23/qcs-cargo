# ADR-0005: Single-replica until measured load crosses these thresholds

Status: Accepted
Date: 2026-04-19

> Note on numbering: the user prompt asked for `0002-single-replica-until.md`,
> but `0002-sqlite-first-migrations.md`, `0003-jwt-access-session-refresh.md`,
> and `0004-observability-prometheus-events-sentry.md` already exist in
> `docs/adr/`. We allocate the next free number, `0005`, to preserve the
> chronological ADR ledger.

## Context

We run a Go Fiber monolith (see ADR-0001) that intentionally co-locates the
HTTP API, static asset delivery, and several background subsystems in a
single process. A non-trivial amount of state is in-process and not safe to
duplicate across replicas without a coordination layer. Until measured load
justifies the operational cost of moving that state out of process, we
commit to running a single replica.

### In-process state inventory (must be moved before adding a second replica)

These subsystems would either misbehave, double-execute, or silently leak
state across replicas if scaled horizontally as-is:

- **Rate limiter** — `internal/middleware/rate_limit.go` keeps a per-process
  in-memory bucket per client. Two replicas means a client effectively
  gets 2x the configured rate budget.
- **Idempotency cache** — `internal/middleware/idempotency.go` is an
  in-memory FIFO LRU per process. A retry that lands on a different
  replica than the original request will execute the side effect twice.
- **Auth throttle prune loop** — `internal/services/auth_throttle.go` runs
  a periodic in-process prune. With N replicas you get N prune loops
  contending on the same rows; the throttle counters themselves also
  live in-process and would diverge.
- **Daily-job tickers** — `cmd/server/main.go` `runDailyJobs` and
  `runOutboundEmailWorker` schedule per-process goroutines. With multiple
  replicas the daily jobs would fire once per replica per day, leading
  to duplicated billing/notification work.
- **MemoryCache** — `internal/services/cache.go` is an in-memory LRU.
  Cache hits/misses and TTL behavior diverge across replicas; no
  cross-replica invalidation exists.
- **Outbound email worker** — `internal/jobs/outbound_email.go` runs
  multiple workers within a single process. **Multi-worker is already
  safe** thanks to the CRIT-03 fix where each claim uses an `:execrows`
  conditional `UPDATE ... WHERE status='pending' AND id=?`, so only one
  worker wins the row. This applies across replicas as well, so the
  outbound email worker is the only listed subsystem that does **not**
  block horizontal scaling on its own.
- **Prometheus registry** — Each replica exposes its own `/metrics` and
  registers its own counters/histograms. Without per-replica labels and
  proper scrape config, dashboards either double-count or randomly
  pick one replica's view.

## Load signals that trigger the migration

Move off the single-replica posture when any of the following is observed
at steady state (i.e. not a transient spike):

- Sustained **>500 req/s over 1 hour** across the API.
- **p95 latency >500ms** at steady state.
- **Single-host CPU >70%** under normal traffic.
- A product decision to support **blue-green deploys**, which inherently
  requires brief dual-running of the old and new versions.

If none of these fire, the operational cost of introducing Redis,
leader election, and a cron host is not yet justified.

## Decision

Run **exactly one replica** of the server binary in production until at
least one of the load signals above is observed. When that happens,
execute the migration paths below before scaling out, not after.

## Migration paths per subsystem

When the time comes, each subsystem has a defined off-ramp. None of these
need to land before they're needed; this ADR records the intended target
so we don't re-litigate the design under pressure.

- **Rate limiter** → move to **Redis** with `INCR`/`PEXPIRE` per
  client+route key.
- **Idempotency cache** → move to **Redis** with the request hash as the
  key and the cached response (or "in-flight" marker) as the value.
- **Daily-job tickers** → either **leader election** among replicas
  (e.g. via a Redis lock or a DB-backed lease row) or move scheduling
  out entirely to an **external cron / systemd timer** that hits a
  protected internal endpoint.
- **MemoryCache** → flip the existing **Redis fallback** so Redis is the
  source of truth across replicas; the in-process LRU becomes a
  best-effort L1.
- **Outbound email worker** → **no change required**. The `:execrows`
  claim race already serializes per-row work safely.
- **Auth throttle prune** → extract into a **separate cron** (or
  leader-elected goroutine) so it runs exactly once cluster-wide.
- **Prometheus** → add a **`replica` label** (or use the pod/instance
  label from the scrape config) so per-replica series are distinct,
  and update dashboards to aggregate across replicas where appropriate.

## Consequences

- **Brief dual-running during blue-green is NOT supported.** A blue-green
  deploy requires both versions to serve at once; today a second
  replica would corrupt rate-limit and idempotency state and
  double-fire daily jobs.
- **Deploy is stop-the-world.** Restarting the single replica drops
  in-flight requests and briefly takes the API offline. Rolling
  deploys are not available without first executing the migration
  paths above.
- **Schema migrations require a maintenance window.** With one replica
  there is no opportunity to drain traffic during a migration; long
  migrations should run during a stated window.
- **Single point of failure.** A crash, OOM, or host failure takes the
  entire API down until the process restarts. Process supervision
  and fast restarts mitigate this but do not replace HA.
- We avoid the operational cost of Redis, leader election, and an
  external scheduler until traffic actually justifies it.
