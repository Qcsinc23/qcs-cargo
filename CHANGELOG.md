# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog, using date-based entries.

## [Unreleased]

### Added
- Added account lifecycle endpoint `POST /api/v1/account/deactivate` and updated dashboard account lifecycle UI for deactivate/delete actions.

### Changed
- Session management UI now preserves current browser session when revoking all other sessions (using `keep_session_id`), with improved error handling.
- Magic-link verify now returns `session_id` alongside `access_token` to support reliable session-management UX.
- Account deletion now anonymizes core personal profile fields and revokes active sessions.
- Auth middleware now rejects non-active accounts (`ACCOUNT_INACTIVE`) for protected routes.

## [2026-02-28]

### Security
- Hardened API protections with stricter auth/session handling, CSRF and rate-limit coverage, and safer server-side error behavior.

### Fixed
- Corrected pricing consistency issues so booking and shipment totals are computed and persisted from unified pricing logic.
- Finalized public tracking behavior and response contract for `GET /api/v1/track/:trackingNumber`.

### Added
- Delivered runtime observability wave: API request analytics/performance event capture, DB-backed observability events, and optional Sentry error capture when configured.
- Added remediation documentation wave: OpenAPI core API spec, ADR set, database schema reference, and changelog maintenance baseline.
