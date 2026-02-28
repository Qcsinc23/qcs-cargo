# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog, using date-based entries.

## [Unreleased]

## [2026-02-28]

### Security
- Hardened API protections with stricter auth/session handling, CSRF and rate-limit coverage, and safer server-side error behavior.

### Fixed
- Corrected pricing consistency issues so booking and shipment totals are computed and persisted from unified pricing logic.
- Finalized public tracking behavior and response contract for `GET /api/v1/track/:trackingNumber`.

### Added
- Delivered runtime observability wave: API request analytics/performance event capture, DB-backed observability events, and optional Sentry error capture when configured.
- Added remediation documentation wave: OpenAPI core API spec, ADR set, database schema reference, and changelog maintenance baseline.
