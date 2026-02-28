# Platform and Scaling Notes

Wave 11 closes the remaining platform/performance gaps with pragmatic runtime support:

- Cache abstraction: [internal/services/cache.go](/Users/sherwyngraham/development/Qcs%20Cargo%20Next/internal/services/cache.go)
  - Memory cache by default
  - Redis + memory tiered cache when `REDIS_URL` is set
- Cached public destinations response
- Readiness/runtime endpoints:
  - `GET /api/v1/platform/readiness`
  - `GET /api/v1/platform/runtime`
- Admin moderation queue:
  - `GET /api/v1/admin/moderation`
  - `POST /api/v1/admin/moderation`
  - `PATCH /api/v1/admin/moderation/:id`
- CDN/header support:
  - `CDN_BASE_URL` is surfaced in runtime/readiness responses and echoed as `X-CDN-Base-URL`
  - Static asset responses now use long-lived cache headers where appropriate
- Image optimization helper:
  - `scripts/optimize-images.sh`

Horizontal scaling readiness:

- API sessions and revocation are already DB-backed.
- Optional Redis allows shared cache behavior across nodes.
- Static assets are cacheable and CDN-friendly.
- Remaining work for true multi-node deployment is operational: externalized SQLite replacement, shared object storage, and background-job leader election.
