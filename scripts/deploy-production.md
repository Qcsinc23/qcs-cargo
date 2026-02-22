# Deploy QCS Cargo to qcs-cargo.com (production server)

## Server

- **Host:** 82.25.85.157
- **Reverse proxy:** Traefik (Docker), ports 80/443, Let's Encrypt
- **Old app:** `/root/qcs-cargo-v2` (SvelteKit + PocketBase) — stopped and replaced by this app

## One-time setup on server

1. Clone the repo (if not already present):
   ```bash
   cd /opt && git clone https://github.com/Qcsinc23/qcs-cargo.git qcs-cargo && cd qcs-cargo
   ```
2. Create `.env` from `.env.example` and set production values:
   - `JWT_SECRET` — long random string (e.g. `openssl rand -base64 32`)
   - `APP_URL=https://qcs-cargo.com`
   - `RESEND_API_KEY`, `FROM_EMAIL`, `STRIPE_*` as needed

## Deploy steps

1. **Pull latest and build**
   ```bash
   cd /opt/qcs-cargo
   git pull origin main
   docker compose -f docker-compose.prod.yml build --no-cache
   ```

2. **Run migrations** (first deploy or after schema changes)
   ```bash
   docker compose -f docker-compose.prod.yml run --rm -e DATABASE_URL=file:/data/qcs.db?_journal_mode=WAL -v qcs_cargo_data:/data qcs-cargo /app/qcs-migrate
   ```
   (If the migrate binary reads DATABASE_URL from env, ensure the volume is mounted at /data and the compose run passes the same env; the compose file mounts qcs_cargo_data at /data, so the default DATABASE_URL in the service is already file:/data/qcs.db. So just run:)
   ```bash
   docker compose -f docker-compose.prod.yml run --rm qcs-cargo /app/qcs-migrate
   ```

3. **Stop old QCS Cargo v2**
   ```bash
   cd /root/qcs-cargo-v2
   docker compose -f docker-compose.prod.yml down
   ```

4. **Start new QCS Cargo**
   ```bash
   cd /opt/qcs-cargo
   docker compose -f docker-compose.prod.yml up -d
   ```

5. **Verify**
   - https://qcs-cargo.com/api/v1/health → `{"status":"ok",...}`
   - https://qcs-cargo.com/ → home page

## Rollback

To revert to the old app:
```bash
 cd /opt/qcs-cargo && docker compose -f docker-compose.prod.yml down
 cd /root/qcs-cargo-v2 && docker compose -f docker-compose.prod.yml up -d
```
