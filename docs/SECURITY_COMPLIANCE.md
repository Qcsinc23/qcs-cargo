# Security and Compliance Additions

Wave 11 adds MVP coverage for the remaining security/compliance findings:

- MFA setup/challenge/verify/disable via `/api/v1/security/mfa/*`
- API key create/list/revoke/rotate via `/api/v1/security/api-keys*`
- Feature flags via `/api/v1/security/feature-flags*`
- Cookie consent capture via `/api/v1/compliance/cookie-consent`
- GDPR export/delete request metadata via `/api/v1/compliance/gdpr/*`
- Resource version history via `/api/v1/compliance/version-history/:resource_type/:resource_id`
- Recipient soft-delete restore via `/api/v1/compliance/recipients/:id/restore`
- IP allow/deny enforcement for `X-API-Key` requests via `middleware.EnforceAPIKeyIPAccess`

Notes:

- MFA is implemented as pragmatic email-OTP/TOTP-style scaffolding. In non-production environments the OTP is echoed in the API response for testability.
- API keys are hashed at rest. Only the one-time raw key is returned at creation/rotation time.
- Version history is stored in `resource_versions` and currently tracks feature-flag changes, GDPR requests, API key lifecycle, cookie consent updates, and recipient restore actions.
- Cookie consent is user-bound and versioned. Essential cookies remain enabled.
