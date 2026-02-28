# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog, using date-based entries.

## [Unreleased]

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
