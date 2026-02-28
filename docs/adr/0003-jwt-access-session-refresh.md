# ADR-0003: JWT Access Tokens with DB-Backed Refresh Sessions

Status: Accepted  
Date: 2026-02-28

## Context

Authentication currently uses:

- Short-lived access JWTs (15 minutes) for bearer auth.
- Refresh JWTs (7 days) in `qcs_refresh` httpOnly cookie (`SameSite=Strict`; `Secure` on HTTPS app URL).
- `JWT_SECRET` is required to be at least 32 characters in production startup checks.

Refresh tokens map to server-side session rows in `sessions` (`refresh_token_hash`, `expires_at`). Logout deletes the refresh session and performs best-effort access-token revocation by storing JWT JTI values in `token_blacklist`.

## Decision

Keep the hybrid model: stateless access JWTs for request auth, plus DB-backed refresh sessions for renewal and revocation control.

## Consequences

- Access-token checks stay fast and simple for API calls.
- Session and token revocation are supported through DB state.
- Refresh and revocation flows depend on database availability.
- Clients must handle both bearer token lifecycle and refresh cookie behavior.
