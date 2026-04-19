# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog, using date-based entries.

## [Unreleased]

## [2026-04-19]

### Changed (Dashboard UX pass 2)
- Migrated every remaining signed-in dashboard tab to the shared `QCSPWA` shell introduced in pass 1: `settings`, `settings/notifications`, `settings/security`, `settings/sessions`, `settings/delete-account`, `profile`, `mailbox`, `inbox`, `inbox-detail`, `inbound`, `inbound-detail`, `ship`, `ship-requests`, `ship-request-detail`, `customs`, `shipments`, `shipment-detail`, `bookings`, `booking-detail`, `booking-wizard`, `invoices`, `invoice-detail`, `recipients`, `templates`. Customer now sees one consistent UI after sign-in.
- `QCSPWA.renderSidebar(activeKey)` is the single source of truth for the customer sidebar; adding or relabeling a tab is now a one-line change in `pwa-shell.js`.
- Every signed-in page now loads on the `qcs-page-wrap` + `qcs-page-main` layout with a skip-link, focuses `<main>` on load, uses `QCSPWA.fetchJson` (auto-refresh on 401, uniform error surface), and renders `QCSPWA.mountLoading` / `mountError` / `renderEmptyState` for the three universal states. Failed loads now surface a real error card with retry instead of a stuck "Loading..." string.
- `ship.html` rebuilt as a 4-step wizard with progress tracker and step-gating; `booking-wizard.html` rebuilt as a 5-step wizard with date + slot picker; `customs.html` shows an explicit locked-state card when the ship request is no longer editable; detail pages got a real two-column layout (details card + actions sidecar).

### Fixed (Dashboard UX pass 2)
- `inbound-detail.html` referenced an undefined `expectedItems` variable that would throw and blank the page when an inbound entry had that field. The value is now read from the response.
- Several pages caught a 401 by hard redirecting to `/login`, dropping the intended URL. Login redirects now preserve the requested path so the user lands back on the page they wanted.

### Changed (Dashboard UX pass 1)
- Customer dashboard review introduced the shared dashboard UX kit on `window.QCSPWA`: `renderSidebar`, `fetchJson` with one-shot 401 refresh, `toast`, `mountError`, `statusBadge`, `formatMoney`, `formatDate`, `safeURL`, `copyToClipboard`, `bindLogout`. Sidebar gained `Parcel+`, `Ship a Package` CTA, consistent emoji icons, and an active-state ring.
- `pwa-shell.css` gained `.qcs-sidebar*`, `.qcs-toast*`, `.qcs-badge*`, `.qcs-error-state`, `.qcs-skip-link`, focus-visible outline, and a `prefers-reduced-motion` override.
- Bulk fix across 21 dashboard pages: `pwa-shell.css` and `pwa-shell.js` are now loaded on every dashboard HTML page (previously missing on settings/, ship.html, customs.html, shipment-detail.html, invoice-detail.html, booking-* and others) so theme/locale toggles, keyboard shortcuts, the utility dock, and toasts work consistently across the app.
- `index.html` (Overview) rewritten with real stat cards backed by `/api/v1/locker/summary`, `/api/v1/inbound-tracking`, `/api/v1/ship-requests`, `/api/v1/shipments`; recent ship requests panel with status badges; storage-fee banner with severity tiers (danger/warning/info); quick actions grid; primary "Ship my packages" CTA.
- `parcel-plus.html` rewritten to escape `sender_name` and pass photo URLs through `QCSPWA.safeURL`, blocking stored `javascript:`/`data:` URLs from ending up in `<img src>`.
- `ship-request-detail.html` escapes `special_instructions` (customer-controlled, also rendered in admin/warehouse views), uses `QCSPWA.statusBadge` + `formatMoney`, and adds a "Pay now" CTA when the request is in `pending_payment`.

### Fixed (Dashboard UX pass 1)
- `/dashboard/shipments/:id` and `/dashboard/invoices/:id` were referenced by their list pages but had no entry in `cmd/server/static_routes.go`, so "View" links 404'd. Added both, with tests.
- `pay.html` and `confirmation.html` were calling `POST /api/v1/ship-requests/:id/reconcile`, which is admin-only and always 403'd for customers. The webhook reconciler from audit fix C-3 already flips `payment_status='paid'` atomically, so the client reconcile call is removed. `pay.html` now honours the C-2 reusable PaymentIntent flow, reports `ALREADY_PAID` 409 cleanly, and uses `formatMoney` for the total label. `confirmation.html` polls the ship request and surfaces distinct paid / review_required / processing cards.

### Security (Pass 2.5 audit remediation)
- **GDPR data erasure**: `accountDelete` now scrubs every PII-bearing user-scoped table (recipients, locker_packages, ship_requests, bookings, customs docs, signatures, photos, MFA, API keys, etc.) via the new `services.AnonymizeUserData` helper. The `compliance/gdpr/delete-request` endpoint runs the same anonymization synchronously and marks the audit row processed. `appendResourceVersion` redacts payloads for `gdpr_request` and `cookie_consent` so the audit trail does not retain the very data the user asked to be deleted.
- **MFA email pipeline wired**: MFA challenge codes are now actually delivered via the outbound-email queue (`mfa_challenge_code` template); previously the code was generated, hashed, persisted, and never sent. TOTP method explicitly returns `IMPLEMENTATION_PENDING` to surface the gap rather than fail silently.
- **Auth throttle hardened**: `services.CheckAndRecordAuthRequest` now fails closed on DB errors and runs check+insert in a single transaction; concurrent bursts can no longer multiply the per-account ceiling. Adds per-bucket count cap of 1000 to bound `auth_request_log` growth.
- **Booking authority lockdown**: `bookingUpdate` no longer accepts customer-supplied `payment_status` and restricts customer-writable status to `{pending, cancelled}`. New admin-only `PATCH /admin/bookings/:id/status` for staff lifecycle transitions.
- **Public blog filters drafts**: `GET /blog/:slug` now SQL-filters `status='published' AND published_at <= now`. Drafts and future-scheduled posts no longer leak via guessable slugs.
- **Warehouse last-write-wins fixed**: bay-move and ship-queue status transitions now require an `expected_status`/`previous_bay`; mismatched value returns 409 instead of silently overwriting concurrent staff actions. `warehouseBaysMove` runs in a single transaction. `warehouseServiceQueueUpdate` now allowlists statuses.
- **Stored-XSS prevention in HTML email**: 11 `Send*` template functions wrapped user-controlled fields in `escapeHTML`; `escapeHTML` now also encodes `'`. Contact-form subject CRLF-stripped against mail-header injection.
- **Push subscriptions**: endpoint UNIQUE constraint replaced `(user_id, endpoint)` with `(endpoint)` so a stolen subscription cannot be re-registered under another account; new migration + handler returns 409 on cross-user conflict.

### Added (Pass 2.5)
- ETag + 304 conditional GET for embed.FS-served assets so revalidating clients no longer re-download `tailwind.css`/`pwa-shell.js` every 5 minutes.
- Outbound email queue stuck-row reaper (`ReapStuckOutboundEmails`); rows in `in_progress` past 5 minutes are reset to `pending` and counted via `qcs_jobs_outbound_email_reaped_total`.
- `failed_email_count` in `GET /api/v1/admin/system-health` for ops visibility on queue health without scraping `/metrics`.
- `GET /api/v1/data/export` expanded from 4 collections to 23 user-scoped collections (full GDPR Article 15 portability), with drift-guard test that fails when a new user-scoped table is added without an export entry.
- `MemoryCache` LRU eviction (cap 4096); previously unbounded.
- Pagination caps on `bookingList`, `invoiceList`, `shipmentList`, `inboundTrackingList`, `recipientsList`.
- Recipient PII validation (name/street/city/phone/apt/delivery_instructions) with HTML metacharacter rejection and length caps.
- Booking double-submit dedup via `IdempotencyMiddleware` on `POST /bookings`.
- GDPR request rate-limit (3 per 24h per user) on `complianceGDPRCreateRequest`.
- Customs document MIME allowlist (`application/pdf`, `image/jpeg`, `image/png`, `image/webp`).
- MFA disable accepts password as alternative step-up to OTP via new `services.VerifyUserPassword` helper.
- CI `static-asset-guards` job: fails on inline `<script>` regression and Tailwind drift.
- Sample systemd `qcs-backup.timer` + `.service` units in `docs/DEPLOYMENT.md`.

### Changed (Pass 2.5)
- `RequireAuth` now reads `role` from the live DB row instead of the JWT claim, so a demoted admin loses access on the next request.
- Daily-jobs supervisor wraps each run in `recover()` and surfaces last-success Unix timestamps in `/metrics` and `/admin/system-health`.
- Expiry-notifier predicates rewritten as ranges; missed daily runs catch up rather than silently dropping the warning.
- `complianceGDPRCreateRequest` for `delete_request` runs anonymization inline.
- `securityFeatureFlagsList` now requires `RequireAdmin`.
- `platformReadiness`/`Runtime` no longer expose `cdn_base_url`/`app_url` (moved to admin-only `/admin/system-health`).
- `invoiceToMap` no longer returns redundant `user_id`.
- `recipientsDelete` preflights `GetRecipientByID` so a wrong id returns 404 instead of opaque 200.
- `warehouseExceptionResolve` rejects unknown action with 400 instead of silently coercing to "matched".
- Contact-form log no longer dumps full PII in production (gated behind `services.AllowDebugAuthArtifacts`).
- Warehouse email-failure logs no longer include user/booking IDs.
- `accountDeactivate` now blacklists the current access token immediately (was already done for `accountDelete`).

### Added
- Added account lifecycle endpoint `POST /api/v1/account/deactivate` and updated dashboard account lifecycle UI for deactivate/delete actions.
- Added security/compliance APIs for MFA, feature flags, API keys, cookie consent, GDPR request metadata, recipient restore, and version history.
- Added parcel-plus APIs for consolidation preview, assisted purchase requests, package photos, customs-document metadata, signature capture, recipient import/export, and loyalty summary.
- Added readiness/runtime endpoints, moderation queue endpoints, cache abstraction, notification stream/push subscription APIs, and a parcel-plus dashboard page.
- Added wave 11 documentation: `docs/SECURITY_COMPLIANCE.md`, `docs/PARCEL_FEATURES.md`, `docs/PWA_UX.md`, and `docs/PLATFORM_SCALING.md`.

### Changed
- Session management UI now preserves current browser session when revoking all other sessions (using `keep_session_id`), with improved error handling.
- Magic-link verify now returns `session_id` alongside `access_token` to support reliable session-management UX.
- Account deletion now anonymizes core personal profile fields and revokes active sessions.
- Auth middleware now rejects non-active accounts (`ACCOUNT_INACTIVE`) for protected routes.
- Public destinations now use the new cache abstraction with invalidation on admin destination updates.
- Static asset responses now emit longer-lived cache headers and optional CDN metadata, and the service worker now supports offline warehouse action replay.

## [2026-02-28]

### Security
- Hardened API protections with stricter auth/session handling, CSRF and rate-limit coverage, and safer server-side error behavior.

### Fixed
- Corrected pricing consistency issues so booking and shipment totals are computed and persisted from unified pricing logic.
- Finalized public tracking behavior and response contract for `GET /api/v1/track/:trackingNumber`.

### Added
- Delivered runtime observability wave: API request analytics/performance event capture, DB-backed observability events, and optional Sentry error capture when configured.
- Added remediation documentation wave: OpenAPI core API spec, ADR set, database schema reference, and changelog maintenance baseline.
