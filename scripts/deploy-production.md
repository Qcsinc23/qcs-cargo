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
   - `RESEND_API_KEY` — required for magic-link and contact-form emails ([resend.com](https://resend.com) API key). Verify the sending domain in Resend and set `FROM_EMAIL` (e.g. `noreply@qcs-cargo.com`).
   - `STRIPE_*` as needed

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

## GitHub Actions auto-deploy

The intended production path is PR-gated:

1. Open a pull request targeting `main`
2. Wait for `CI` to pass
3. Merge the PR into `main`
4. GitHub Actions runs the production deploy automatically after successful post-merge `CI`

1. Add these repository secrets in GitHub:
   - `PROD_SSH_PRIVATE_KEY` — private SSH deploy key for the non-root deploy user
   - `PROD_SSH_KNOWN_HOSTS` — pinned `known_hosts` entry for the production host
2. Add this repository variable in GitHub:
   - `PROD_USER=deploy`
2. The workflow [.github/workflows/deploy.yml](../.github/workflows/deploy.yml) will:
   - wait for the `CI` workflow to succeed on `main`
   - SSH to the non-root deploy user on `82.25.85.157`
   - create a clean detached git worktree for the exact validated commit SHA
   - run [`scripts/deploy-production.sh`](deploy-production.sh)
   - verify `https://qcs-cargo.com/api/v1/health` and `https://qcs-cargo.com/`

The server-side script uses a lock file under `.deploy/` to prevent overlapping deployments and waits for the Docker health check to report `healthy` before marking the deployment successful. The temporary worktree keeps dirty local files in `/opt/qcs-cargo` from blocking deploys, and the workflow retries transient SSH transport failures before marking the deploy failed.

## Traefik file provider (this server)

On this host, Traefik also loads a **file** config at `/etc/dokploy/traefik/dynamic/qcs-cargo.yml` that routes `qcs-cargo.com` / `www.qcs-cargo.com` to the service. That file was updated to point to `http://qcs_cargo:8080` (replacing the old `qcs_web:3000`). If you deploy to a fresh server with Dokploy/Traefik, either add a similar file or rely only on the Docker labels in `docker-compose.prod.yml` (and ensure no conflicting file exists).

## Rollback

To revert to the old app:
```bash
 cd /opt/qcs-cargo && docker compose -f docker-compose.prod.yml down
 # Restore Traefik file so it points to v2 again:
 sudo cp /etc/dokploy/traefik/dynamic/qcs-cargo.yml.bak /etc/dokploy/traefik/dynamic/qcs-cargo.yml
 cd /root/qcs-cargo-v2 && docker compose -f docker-compose.prod.yml up -d
```
