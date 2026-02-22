# QCS Cargo — Deployment

## Prerequisites

- **Go 1.22+**
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
| `RESEND_API_KEY` | Resend API key for transactional email (magic link, contact form) |
| `STRIPE_SECRET_KEY` | Stripe secret key for payments |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook signing secret |
| `APP_URL` | Public base URL of the app (e.g. `https://app.qcs-cargo.com`) |
| `FROM_EMAIL` | Sender address for outgoing email |
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
- **CORS**: Set `ALLOWED_ORIGINS` to your frontend origin(s), e.g. `https://app.qcs-cargo.com`. Leave empty only for local development.
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
