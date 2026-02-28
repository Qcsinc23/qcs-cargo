# QCS Cargo — Deployment

## Prerequisites

- **Go 1.25+**
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

Automatic deployments are supported through [.github/workflows/deploy.yml](../.github/workflows/deploy.yml). Configure the repository secret `PROD_SSH_PASSWORD` so pushes to `main` deploy automatically after the `CI` workflow completes successfully.
