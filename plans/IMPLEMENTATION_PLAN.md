# QCS Cargo — Implementation Plan (Reviewed & Adjusted)

**Source:** [`AUDIT_REPORT.md`](AUDIT_REPORT.md)  
**Audit Date:** 2026-02-28  
**Plan Revision Date:** 2026-02-28  
**Total Findings in Scope:** 120

**Execution Progress Snapshot (2026-02-28):**
- **Implemented:** 83
- **Open:** 37
- **Source of truth:** [`findings_status.md`](../findings_status.md)

---

## 1) Executive Intent

This revised plan converts the audit into a delivery roadmap that is:

- **Risk-first** (security and integrity before feature expansion)
- **Dependency-aware** (database and auth primitives before UI and UX layers)
- **Test-gated** (every phase includes validation and rollback criteria)
- **Operationally realistic** (avoids fragile implementation patterns and incorrect snippets)

---

## 2) Key Corrections Applied to Previous Plan

The prior version had useful direction, but several implementation snippets were either risky, inaccurate for Fiber/Go, or not aligned with the audit’s exact intent. This revision fixes that by changing the plan at the strategy and task-definition level.

### 2.1 Corrected security behavior expectations

- **BUG-001**: No runtime hardcoded JWT secret fallback in production paths; startup validation required.
- **BUG-004**: Clarified that token must never be returned to client responses; controlled dev logging only.
- **INC-015**: Production CORS must require explicit origins; dev convenience is allowed but isolated.
- **INC-016**: Refresh cookie policy tightened and documented for app behavior.

### 2.2 Corrected implementation approach risks

- Removed dependence on app-level ad hoc object stores for rate limiting and replaced with middleware-driven architecture.
- Removed brittle CSRF design assumptions; added explicit route scoping and browser-session boundaries.
- Corrected observability approach for Sentry so it integrates with Fiber request lifecycle safely.
- Added missing migration safety requirements (idempotence, rollback feasibility, and data backfill notes).

### 2.3 Added missing delivery controls

- Phase exit criteria per tranche.
- Test matrix requirements per phase.
- Rollout/rollback strategy for auth, pricing, and schema updates.
- Full issue-coverage mapping so all 120 findings are accounted for.

---

## 3) Guiding Constraints

1. **No breaking auth changes without transition path** (cookie/token/session compatibility).
2. **No schema change without migration + backward compatibility window**.
3. **No security control added globally without endpoint allow/deny list** (webhooks/public endpoints).
4. **No “single fix” merges without tests for positive and negative paths**.
5. **All critical/high findings must have traceable closure evidence** (PR, test, and verification artifact).

---

## 4) Delivery Phases

## Phase 0 — Stabilization & Delivery Guardrails (Day 0–1)

**Objective:** Make subsequent remediation safe and observable.

### Scope

- Lock baseline branch and create remediation epic.
- Add risk labels per severity and category.
- Define environment contracts for secrets and runtime config.
- Confirm Go version policy and CI matrix target.

### Findings covered

- CQ-007 (Go version validity)
- Process prerequisite for all remaining issues

### Acceptance criteria

- [ ] Project build/test baseline captured.
- [ ] Environment variable contract documented.
- [ ] CI branch protection in place for remediation branches.

---

## Phase 1 — Immediate Security & Authentication Hardening (Week 1)

**Objective:** Remove high-probability exploit paths first.

### 1.1 Authentication and token integrity

**Findings:** BUG-001, BUG-002, BUG-004, INC-016, INC-017, MISS-005, MISS-031

#### Tasks

- Enforce startup validation for JWT secret requirements.
- Implement dedicated auth endpoint rate limiting (per-IP + per-identity dimension).
- Remove magic-link token exposure from API responses.
- Harden refresh cookie policy and document same-site behavior impacts.
- Implement token revocation strategy (blacklist or equivalent short-lived approach with revocation guarantees).
- Add security headers middleware with environment-aware HSTS behavior.

#### Acceptance criteria

- [ ] No auth token material is returned beyond intended credentials.
- [ ] Auth abuse thresholds enforced and tested.
- [ ] Revoked access token is rejected before expiry.
- [ ] Security headers present on protected/public pages as designed.

### 1.2 Browser/API trust boundaries

**Findings:** INC-015, INC-018, INC-020

#### Tasks

- Enforce explicit production CORS origin configuration.
- Introduce CSRF protection only for cookie-authenticated state-changing browser flows.
- Ensure request ID is echoed in response headers for traceability.

#### Acceptance criteria

- [ ] Production startup fails without explicit CORS origin list.
- [ ] CSRF checks do not break webhooks or machine-to-machine endpoints.
- [ ] `X-Request-ID` visible in all API responses.

### 1.3 Credential and identity data validation

**Findings:** INC-019, INC-031, MISS-001, MISS-007, INC-012

#### Tasks

- Enforce password complexity policy consistently across register/reset/change.
- Enforce email format and normalization (lowercase canonical storage).
- Implement required email verification before account activation.
- Add DB- and app-level protections against duplicate identity records.

#### Acceptance criteria

- [ ] Weak passwords rejected with deterministic validation errors.
- [ ] Email normalization applied before uniqueness checks.
- [ ] Unverified users cannot authenticate into protected areas.

### Phase 1 exit gate

- [ ] Pen-test smoke checklist passes for auth abuse, CSRF, token replay, and CORS.
- [ ] Critical security findings from immediate roadmap resolved or explicitly accepted with compensating controls.

---

## Phase 2 — Core Correctness, Data Integrity, and Critical UX Flows (Weeks 2–4)

**Objective:** Eliminate inconsistent business behavior and high-impact functional defects.

### 2.1 Pricing and booking correctness

**Findings:** BUG-005, BUG-006, BUG-009, INC-006, INC-033, CQ-005

#### Tasks

- Consolidate shipping/pricing logic into one source of truth.
- Remove divergent calculation pathways and update all callers.
- Fix booking update recalculation using persisted values for omitted fields.
- Validate destination IDs and booking date constraints.
- Replace magic numbers with named constants in pricing-related paths.

#### Acceptance criteria

- [ ] Same input yields same total across calculator, booking, and ship request flows.
- [ ] Partial booking updates do not zero missing dimensions.
- [ ] Invalid destination/date inputs fail early with clear errors.

### 2.2 Shipping lifecycle consistency

**Findings:** BUG-007, INC-007, BUG-010, BUG-012

#### Tasks

- Implement public tracking endpoint behavior fully.
- Correct active/inactive ship-request filtering for reship eligibility.
- Add payment max-boundary or secondary confirmation logic for high values.
- Reject invalid numeric parsing in public calculator flow.

#### Acceptance criteria

- [ ] Public tracking returns valid state for shipment and confirmation contexts.
- [ ] Completed/cancelled requests no longer block valid operations.
- [ ] Payment and parsing guards prevent pathological inputs.

### 2.3 DB schema and migration hardening

**Findings:** INC-011, INC-013, INC-014, INC-043, BUG-013

#### Tasks

- Add unique suite-code index with safe migration sequence.
- Add status CHECK constraints where missing.
- Add status index coverage for warehouse query patterns.
- Add down migrations and migration verification process.
- Stop ignoring DB PRAGMA/configuration errors.

#### Acceptance criteria

- [ ] Constraint violations are deterministic and handled by API.
- [ ] Migration up/down tested in CI for new changes.
- [ ] Query plans demonstrate expected index use on hot paths.

### 2.4 Auth/session product completeness

**Findings:** MISS-003, MISS-004, MISS-006, INC-021

#### Tasks

- Add session management API/UI (list/revoke).
- Implement account deletion/deactivation with compliant anonymization strategy.
- Log auth/admin-sensitive operations to activity trail.

#### Acceptance criteria

- [ ] Users can self-manage sessions safely.
- [ ] Deletion flow is explicit, reversible only by policy, and auditable.
- [ ] Auth/admin events are queryable in activity logs.

### 2.5 Reliability and observability foundation

**Findings:** MISS-041, MISS-043, MISS-044, INC-027, INC-028, INC-029, BUG-015

#### Tasks

- Integrate centralized error tracking with release/environment metadata.
- Improve async/email error handling visibility and retry strategy.
- Add baseline metrics and alerting hooks.

#### Acceptance criteria

- [ ] Unhandled errors produce structured telemetry.
- [ ] Email delivery failures are measurable and actionable.
- [ ] Core business metrics are emitted and reviewable.

### 2.6 CI/CD and quality pipeline

**Findings:** INC-034, INC-035, INC-036, INC-037, INC-038, INC-039, INC-041, CQ-006

#### Tasks

- Add CI workflow for lint, unit tests, integration tests, and build artifacts.
- Add vulnerability scanning (`govulncheck`) and dependency checks.
- Expand integration and E2E coverage for critical user journeys.

#### Acceptance criteria

- [ ] PRs blocked on failing lint/test/security jobs.
- [ ] Integration suite covers auth, bookings, ship requests, and warehouse basics.
- [ ] Critical E2E flows executed in CI at minimum smoke depth.

### Phase 2 exit gate

- [ ] High-severity correctness findings closed for pricing, booking, and ship lifecycle.
- [ ] CI/CD baseline operational and enforced.

---

## Phase 3 — Product Completeness and Platform Maturity (Quarter)

**Objective:** Deliver medium-term capabilities and architectural debt reduction.

### 3.1 Frontend completion and UX essentials

**Findings:** INC-022, MISS-008, MISS-009, MISS-010, MISS-011, MISS-014

#### Tasks

- Complete WASM frontend from placeholder to routed, stateful app.
- Implement loading/empty/error states comprehensively.
- Execute accessibility remediation plan (WCAG 2.1 AA baseline).
- Introduce keyboard navigation and theme support roadmap.

### 3.2 Operational and compliance capabilities

**Findings:** MISS-032, MISS-033, MISS-034, MISS-037, MISS-038, MISS-039, MISS-040

#### Tasks

- Add cookie consent and compliance workflow support.
- Add dependency scanning automation and policy enforcement.
- Produce API docs, ADRs, schema docs, and changelog discipline.

### 3.3 Architecture and performance

**Findings:** INC-004, CQ-008, CQ-009, CQ-010, MISS-024, MISS-026, MISS-027, MISS-028, MISS-029

#### Tasks

- Refactor DB access patterns for better testability/isolation.
- Address N+1 and search pagination/performance hotspots.
- Add response compression and selective caching.
- Improve health monitoring surface and dashboard visibility.

### 3.4 Parcel-forwarding differentiators

**Findings:** MISS-016, MISS-018, MISS-019, MISS-020, MISS-021, MISS-022, MISS-045, MISS-047, MISS-048, MISS-049, MISS-050, MISS-051

#### Tasks

- Implement in-app notifications and real-time updates.
- Add export/import workflows with validation and auditability.
- Deliver consolidation preview and package-photo experience.
- Add customs/document and delivery-proof enhancements.

### Phase 3 exit gate

- [ ] Medium-severity architectural and product gaps closed or committed with dated milestones.

---

## Phase 4 — Long-Term Strategic Enhancements (Next Quarter+)

**Objective:** Build competitive moat and scale resilience.

### Scope

- MFA/2FA expansion and account security UX polish (MISS-002).
- Offline warehouse operation capabilities (MISS-012).
- Internationalization and regionalization support (MISS-015).
- Feature flags and controlled rollout mechanics (MISS-023).
- Advanced analytics and user-insight capabilities (MISS-042).
- Loyalty and engagement systems (MISS-052).
- Horizontal scaling preparedness and distributed architecture readiness (MISS-030).

### Phase 4 exit gate

- [ ] Strategic initiatives tied to measurable business outcomes and SLO impact.

---

## 5) Full Finding Coverage Matrix (All 120)

## 5.1 Bugs (15)

- **Phase 1:** BUG-001, BUG-002, BUG-003, BUG-004
- **Phase 2:** BUG-005, BUG-006, BUG-007, BUG-009, BUG-010, BUG-012, BUG-013, BUG-014, BUG-015
- **Phase 3+:** BUG-008, BUG-011

## 5.2 Incorrect Implementations (22)

- **Phase 1:** INC-001, INC-015, INC-016, INC-017, INC-018, INC-019, INC-020
- **Phase 2:** INC-006, INC-007, INC-011, INC-012, INC-013, INC-014, INC-021
- **Phase 3+:** INC-002, INC-003, INC-004, INC-005, INC-008, INC-009, INC-010

## 5.3 Incomplete Implementations (21)

- **Phase 2:** INC-023, INC-027, INC-028, INC-029, INC-030, INC-032, INC-033, INC-034, INC-035, INC-036, INC-037, INC-038, INC-039, INC-040, INC-041, INC-043
- **Phase 3+:** INC-022, INC-024, INC-025, INC-026, INC-042

## 5.4 Missing Features (52)

- **Phase 1:** MISS-001, MISS-005, MISS-007, MISS-031
- **Phase 2:** MISS-003, MISS-004, MISS-006, MISS-041
- **Phase 3:** MISS-008, MISS-009, MISS-010, MISS-011, MISS-013, MISS-014, MISS-016, MISS-017, MISS-018, MISS-019, MISS-020, MISS-021, MISS-022, MISS-024, MISS-026, MISS-027, MISS-028, MISS-029, MISS-032, MISS-033, MISS-034, MISS-035, MISS-036, MISS-037, MISS-038, MISS-039, MISS-040, MISS-042, MISS-043, MISS-044, MISS-045, MISS-046, MISS-047, MISS-048, MISS-049, MISS-050, MISS-051
- **Phase 4+:** MISS-002, MISS-012, MISS-015, MISS-023, MISS-025, MISS-030, MISS-052

## 5.5 Code Quality (10)

- **Phase 2:** CQ-005, CQ-006, CQ-007
- **Phase 3+:** CQ-001, CQ-002, CQ-003, CQ-004, CQ-008, CQ-009, CQ-010

---

## 6) Test and Verification Plan

## 6.1 Security verification

- Auth endpoint abuse tests (rate limits, lockout behavior, replay checks)
- CSRF bypass attempts across browser-authenticated routes
- CORS origin and preflight verification in prod-like config
- Header scanner checks for CSP/HSTS/frame/content-type policy

## 6.2 Correctness verification

- Deterministic pricing golden tests for all service and destination permutations
- Booking partial update tests to validate merge semantics
- Public tracking contract tests for all state paths

## 6.3 Data and migration verification

- Migration up/down test runs on clean and seeded DBs
- Constraint/index behavior checks with representative workloads

## 6.4 Observability verification

- Error tracking smoke tests with synthetic exceptions
- Request ID trace continuity tests across logs/responses
- Email failure path validation and alert trigger checks

---

## 7) Rollout & Risk Management

- Use feature flags (where possible) for auth/session behavior changes.
- Deploy schema changes before application behavior that depends on them.
- For pricing consolidation, run shadow-calculation comparison logs before full cutover.
- For session/token revocation, monitor auth failure rates and rollback quickly if abnormal.

---

## 8) Definition of Done (Per Finding)

A finding is considered complete only when all are true:

- [ ] Code change merged with linked finding ID.
- [ ] Unit/integration tests added or updated.
- [ ] Security/correctness verification artifact attached.
- [ ] Operational documentation updated where behavior changes.
- [ ] Audit finding status updated to **Closed** (or **Accepted Risk** with rationale).

---

## 9) Immediate Next Sprint Backlog (Recommended)

1. BUG-001, BUG-002, BUG-004, INC-015, INC-016, MISS-031
2. INC-018, INC-020, INC-019, INC-031, BUG-003
3. BUG-005, BUG-006, BUG-009, INC-006, INC-033
4. INC-035, INC-039, CQ-006, MISS-041

This sequence minimizes exploit risk first, then stabilizes core business correctness and release confidence.
