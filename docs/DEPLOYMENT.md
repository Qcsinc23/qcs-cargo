# QCS Cargo — Deployment

## Prerequisites

- **Go 1.26.1+**
- **SQLite** (used via `modernc.org/sqlite`; no separate install required on most systems)

## Build

```bash
make build
```

Produces:

- `qcs-server` — main API and web server
- `qcs-migrate` — database migrations

## Environment

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | SQLite connection string (default: `file:qcs.db?_journal_mode=WAL`) |
| `JWT_SECRET` | Secret for signing JWT tokens (required for auth) |
| `PORT` | HTTP port (default: `8080`) |
| `RESEND_API_KEY` | **Required for magic links.** Resend API key for transactional email (magic link, contact form, password reset). Get a key at [resend.com/api-keys](https://resend.com/api-keys). If unset, magic-link and contact-form emails are not sent (link is only logged server-side). |
| `FROM_EMAIL` | Sender address for transactional email (e.g. `noreply@qcs-cargo.com`). Must be from a domain you have verified in the Resend dashboard. |
| `STRIPE_SECRET_KEY` | Stripe secret key for payments |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook signing secret |
| `APP_URL` | Public base URL of the app (production: `https://qcs-cargo.com`) |
| `APP_ENV` | Runtime environment. Set to `production` in production deployments so auth, CORS, and other hardening paths do not fall back to development behavior. |
| `UPLOAD_DIR` | Directory for uploaded files (default: `./uploads`) |
| `ALLOWED_ORIGINS` | Optional. Comma-separated CORS origins; empty = allow all (dev only) |

## Run

```bash
./qcs-server
```

## Migrations

Run after deployment or when schema changes:

```bash
./qcs-migrate
```

## Production

- **HTTPS**: Serve behind a reverse proxy (e.g. nginx, Caddy) with TLS. Do not expose the Go server directly on the internet.
- **CORS**: Set `ALLOWED_ORIGINS` to your site origin(s), e.g. `https://qcs-cargo.com`. Leave empty only for local development.
- **JWT**: Use a long, random `JWT_SECRET` and keep it out of version control.
- **APP_ENV**: Set `APP_ENV=production`. The checked-in production compose file now enforces this and the deploy script refuses to continue if it is missing.
- **Email**: Do not rely on log-only auth link delivery in production. Configure `RESEND_API_KEY` and `FROM_EMAIL` before exposing auth flows.

### Optional: systemd unit

```ini
[Unit]
Description=QCS Cargo server
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/qcs-cargo
ExecStart=/opt/qcs-cargo/qcs-server
Restart=on-failure
EnvironmentFile=/opt/qcs-cargo/.env

[Install]
WantedBy=multi-user.target
```

### Scheduled backups

The `make backup` target captures a hot snapshot of the SQLite database
and ships it off-host (when `BACKUP_REMOTE_DEST` is set). To run it on a
schedule via systemd, drop the following service + timer pair next to
the `qcs-server.service` unit above:

`/etc/systemd/system/qcs-backup.service`:

```ini
[Unit]
Description=QCS Cargo SQLite backup
After=network.target qcs-server.service

[Service]
Type=oneshot
User=qcs
WorkingDirectory=/opt/qcs-cargo
EnvironmentFile=/opt/qcs-cargo/.env
ExecStart=/usr/bin/make backup
```

`/etc/systemd/system/qcs-backup.timer`:

```ini
[Unit]
Description=Run QCS Cargo SQLite backup nightly

[Timer]
OnCalendar=*-*-* 02:30:00
Persistent=true
RandomizedDelaySec=300
Unit=qcs-backup.service

[Install]
WantedBy=timers.target
```

Enable with:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now qcs-backup.timer
sudo systemctl list-timers qcs-backup.timer
```

Verify:

```bash
systemctl status qcs-backup.timer
journalctl -u qcs-backup.service --since '1 hour ago'
ls -l /opt/qcs-cargo/.deploy/backups/   # or BACKUP_REMOTE_DEST
```

Notes:

- `User=qcs` should match the user that owns `/opt/qcs-cargo` and the
  database file. Adjust to match your `qcs-server.service` `User=`.
- `Persistent=true` makes systemd run a missed backup once on the next
  boot if the host was off at 02:30. `RandomizedDelaySec=300` spreads
  the load when many hosts share one off-host backup target.
- The timer only triggers `make backup`; off-host shipping still relies
  on `BACKUP_REMOTE_DEST` being set in `/opt/qcs-cargo/.env`. See
  *Off-host shipping* below.

### Optional: Docker one-liner

```bash
docker run -d --name qcs-cargo \
  -p 8080:8080 \
  -e DATABASE_URL=file:/data/qcs.db \
  -e JWT_SECRET=your-secret \
  -v /path/to/data:/data \
  your-registry/qcs-cargo:latest
```

Replace `your-registry/qcs-cargo:latest` with your built image; ensure migrations have been run (e.g. via an init container or pre-deploy step).

## Transactional email (Resend)

Magic link and contact-form emails are sent only when **`RESEND_API_KEY`** is set.

1. **Get an API key**: [resend.com](https://resend.com) → API Keys → Create. Add it to your `.env` as `RESEND_API_KEY=re_...`.
2. **Verify your domain**: In Resend, add and verify the domain you send from (e.g. `qcs-cargo.com`). Set `FROM_EMAIL` to an address on that domain (e.g. `noreply@qcs-cargo.com`).
3. **Restart the app** after adding or changing the key.

**Troubleshooting: "I never received the magic link"**

- Check server logs on startup: you should see `Resend: configured (transactional email enabled)`. If you see `Resend: not configured (magic link and contact form will log only)`, add `RESEND_API_KEY` to `.env` and restart.
- When Resend is not configured, each magic-link request is logged with the one-time link (search logs for `[Resend] not configured`).
- If the key is set but mail still doesn’t arrive: check Resend dashboard for bounces/errors; ensure the sending domain is verified and `FROM_EMAIL` uses that domain.

## Production at qcs-cargo.com (Traefik)

The server uses **Traefik** (Docker) for HTTPS and routing. See **scripts/deploy-production.md** for step-by-step deploy: clone repo to `/opt/qcs-cargo`, build with `docker-compose.prod.yml`, run migrations, stop old app (`/root/qcs-cargo-v2`), start new stack. Traefik routes `Host(qcs-cargo.com)` and `Host(www.qcs-cargo.com)` to the new container (port 8080).

Automatic deployments are supported through [.github/workflows/deploy.yml](../.github/workflows/deploy.yml). Configure these GitHub secrets and variables before enabling auto-deploy:

- Repository secret `PROD_SSH_PRIVATE_KEY`: the deploy key for the production host.
- Repository secret `PROD_SSH_KNOWN_HOSTS`: the pinned `known_hosts` entry for the production host. Do not rely on `ssh-keyscan` during deploy.
- Repository variable `PROD_USER`: a non-root SSH user with access to `/opt/qcs-cargo` and permission to run Docker commands.
- Optional repository variables `PROD_HOST`, `PROD_APP_DIR`, and `PROD_PUBLIC_URL` if the defaults need to change.

The deploy workflow now:

- expects changes to reach `main` through a reviewed pull request after CI passes
- requires a non-root SSH user
- enforces strict host-key checking from the pinned `PROD_SSH_KNOWN_HOSTS` secret
- runs only after the `CI` workflow succeeds or via manual dispatch
- retries transient SSH transport failures before failing the deploy
- deploys from a clean detached git worktree so uncommitted files on the host do not block the release
- keeps deploy lock/state files under `.deploy/` so the non-root deploy user owns them
- reuses `/opt/qcs-cargo/.env` and a fixed Compose project name when deploying from a detached worktree
- performs internal container health checks plus public endpoint checks
- records the last successful Git SHA and attempts a best-effort rollback to that SHA if a deployment fails

Rollback is intentionally code-only. The script does not run down migrations. If a release includes backward-incompatible schema changes, restore the database from backup before rolling the application back.

## Database backup and restore

Audit Phase 1.4 (OPS): the SQLite database is the system of record for accounts, packages, payments, and shipments, and the deploy script does not roll back migrations. A reliable backup procedure is therefore mandatory before every deploy and operationally before every business day.

### Online hot backup

The production server uses SQLite in WAL mode. Use `sqlite3 .backup` (not `cp`) to capture a consistent snapshot while the server is up:

```bash
# On the production host
make backup                                  # writes ./backups/qcs-<UTC>.db + .sha256
DATABASE_PATH=/opt/qcs-cargo/qcs.db \
BACKUP_DIR=/var/backups/qcs make backup      # explicit paths
```

`make backup` runs `PRAGMA integrity_check` against the snapshot and refuses to keep it on a bad result. A `.sha256` companion file is written next to each snapshot so off-host integrity can be re-verified.

### Off-host shipping (recommended)

A backup that lives only on the production host is not a real backup. The deploy script ships the pre-deploy snapshot to a remote destination when `BACKUP_REMOTE_DEST` is set:

```bash
# In /opt/qcs-cargo/.env on the production host
BACKUP_REMOTE_DEST=backups@backup.example.com:/var/backups/qcs/
```

The deploy script will rsync each pre-deploy snapshot (and its `.sha256`) to that destination, fail the deploy if the rsync fails, and prune local snapshots older than `BACKUP_RETENTION_DAYS` (default 30) on success.

### Pre-deploy snapshot

`scripts/deploy-production.sh` snapshots the database with `sqlite3 .backup` before running `qcs-migrate`. The deploy aborts if the snapshot or its integrity check fails, so a broken backup environment cannot silently produce an unrecoverable migration. Set `SKIP_BACKUP=1` only for the very first deploy where no `qcs.db` exists yet.

### Restore procedure

1. Stop the app stack on the production host:
   ```bash
   docker compose -p qcs-cargo --env-file /opt/qcs-cargo/.env -f /opt/qcs-cargo/docker-compose.prod.yml stop
   ```
2. Verify the candidate backup is intact:
   ```bash
   BACKUP=/var/backups/qcs/qcs-20260418T120000Z-pre-abc123def456.db make restore-check
   ```
3. Move the live DB out of the way and restore the snapshot:
   ```bash
   sudo mv /opt/qcs-cargo/qcs.db   /opt/qcs-cargo/qcs.db.bad
   sudo mv /opt/qcs-cargo/qcs.db-shm /opt/qcs-cargo/qcs.db-shm.bad 2>/dev/null || true
   sudo mv /opt/qcs-cargo/qcs.db-wal /opt/qcs-cargo/qcs.db-wal.bad 2>/dev/null || true
   sudo cp /var/backups/qcs/qcs-...db /opt/qcs-cargo/qcs.db
   sudo chown <PROD_USER>:<PROD_USER> /opt/qcs-cargo/qcs.db
   ```
4. Restart the app stack and verify health:
   ```bash
   docker compose -p qcs-cargo --env-file /opt/qcs-cargo/.env -f /opt/qcs-cargo/docker-compose.prod.yml up -d
   curl -fsS https://qcs-cargo.com/api/v1/health
   ```
5. If the restored snapshot predates a migration that the deployed binary requires, either redeploy the binary that matches the snapshot SHA (recorded in the backup filename) or run the necessary forward migrations manually.

### Verification cadence

- Run `make restore-check BACKUP=...` against the most recent backup at least weekly on a non-production host. A backup that has never been restored is a hypothesis, not a backup.
- Confirm `BACKUP_REMOTE_DEST` continues to receive new snapshots after every deploy by checking the remote directory listing.

### How to provision the operator-side values

Two environment variables on the production host (`/opt/qcs-cargo/.env`) unlock off-host DR and error tracking. Both default to unset; the server tolerates both missing values, but running without them is accepted risk, not zero risk.

- `BACKUP_REMOTE_DEST` — rsync target that receives every pre-deploy snapshot. Any rsync-reachable destination works: another VM, an object-storage gateway, `rsync.net`, or your colocation backup box. Recommended shape: a different host in a different failure domain from the production VM. Once set, `scripts/deploy-production.sh` automatically ships new snapshots on every deploy. Leaving it unset means all snapshots live on the same disk as the database — a single-disk failure is total data loss.
- `SENTRY_DSN` — Sentry project DSN. Provision a free-tier Sentry project at [sentry.io](https://sentry.io) (no code change required; the `sentry-go` hook is already wired in `cmd/server/main.go`). Copy the DSN from Settings → Projects → Client Keys (DSN) and paste it here. Leaving it unset means Go runtime panics and handler errors are only written to the container log, which requires pulling logs manually to diagnose any production issue.

After editing `.env`, restart the app stack:

```bash
docker compose -p qcs-cargo --env-file /opt/qcs-cargo/.env -f /opt/qcs-cargo/docker-compose.prod.yml up -d
```
