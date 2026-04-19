#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/qcs-cargo}"
TARGET_BRANCH="${TARGET_BRANCH:-main}"
DEPLOY_REF="${DEPLOY_REF:-origin/$TARGET_BRANCH}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-qcs-cargo}"
SERVICE_NAME="${SERVICE_NAME:-qcs-cargo}"
CONTAINER_NAME="${CONTAINER_NAME:-qcs_cargo}"
APP_URL="${APP_URL:-https://qcs-cargo.com}"
DEPLOY_STATE_DIR="${DEPLOY_STATE_DIR:-$APP_DIR/.deploy}"
LAST_SUCCESSFUL_SHA_FILE="${LAST_SUCCESSFUL_SHA_FILE:-$DEPLOY_STATE_DIR/last-successful.sha}"
LOCK_FILE="${LOCK_FILE:-$DEPLOY_STATE_DIR/production-deploy.lock}"
ENV_FILE="${ENV_FILE:-$APP_DIR/.env}"
HEALTH_RESPONSE_FILE="${HEALTH_RESPONSE_FILE:-$DEPLOY_STATE_DIR/health-check.json}"
HOME_SNAPSHOT_FILE="${HOME_SNAPSHOT_FILE:-$DEPLOY_STATE_DIR/home.html}"
HEALTH_RETRIES="${HEALTH_RETRIES:-20}"
HEALTH_SLEEP_SECONDS="${HEALTH_SLEEP_SECONDS:-3}"
BUILD_FLAGS="${BUILD_FLAGS:---pull}"
ROLLBACK_ON_FAILURE="${ROLLBACK_ON_FAILURE:-1}"
CURL_MAX_TIME="${CURL_MAX_TIME:-10}"
# Phase 1.4 (OPS) backup config. The deploy now snapshots the SQLite
# database with the official .backup command before running migrations,
# and optionally ships the snapshot to an off-host destination via rsync.
DB_PATH="${DB_PATH:-$APP_DIR/qcs.db}"
BACKUP_DIR="${BACKUP_DIR:-$DEPLOY_STATE_DIR/backups}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
# When set, rsync the new snapshot to this destination (e.g.
# "user@backup-host:/var/backups/qcs/"). Optional.
BACKUP_REMOTE_DEST="${BACKUP_REMOTE_DEST:-}"
SKIP_BACKUP="${SKIP_BACKUP:-0}"

mkdir -p "$DEPLOY_STATE_DIR"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "another deployment is already running"
  exit 1
fi

cd "$APP_DIR"

if [[ "${SKIP_GIT_PREP:-0}" != "1" ]]; then
  git fetch origin "$TARGET_BRANCH"
  git checkout --detach "$DEPLOY_REF"
fi

previous_successful_sha="${PREVIOUS_SUCCESSFUL_SHA:-}"
if [[ -f "$LAST_SUCCESSFUL_SHA_FILE" ]]; then
  previous_successful_sha="$(tr -d '[:space:]' < "$LAST_SUCCESSFUL_SHA_FILE")"
fi
if [[ -z "$previous_successful_sha" ]]; then
  previous_successful_sha="$(git rev-parse HEAD)"
fi

deployed_sha="$(git rev-parse HEAD)"
read -r -a build_flags <<< "$BUILD_FLAGS"
echo "deploying_sha=$deployed_sha"
echo "previous_successful_sha=$previous_successful_sha"

compose() {
  docker compose -p "$COMPOSE_PROJECT_NAME" --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

require_production_compose() {
  if ! grep -Eq '^[[:space:]]*-[[:space:]]*APP_ENV=production([[:space:]]*)$' "$COMPOSE_FILE"; then
    echo "production compose must set APP_ENV=production before deploy" >&2
    exit 1
  fi
}

wait_for_container_health() {
  local health_status=""
  for _ in $(seq 1 "$HEALTH_RETRIES"); do
    health_status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$CONTAINER_NAME" 2>/dev/null || true)"
    if [[ "$health_status" == "healthy" ]]; then
      return 0
    fi
    if [[ "$health_status" == "dead" || "$health_status" == "exited" ]]; then
      docker logs --tail 200 "$CONTAINER_NAME" >&2 || true
      echo "container failed with state=$health_status" >&2
      return 1
    fi
    sleep "$HEALTH_SLEEP_SECONDS"
  done
  compose ps >&2 || true
  docker logs --tail 200 "$CONTAINER_NAME" >&2 || true
  echo "container did not become healthy" >&2
  return 1
}

verify_public_endpoints() {
  curl --fail --silent --show-error --max-time "$CURL_MAX_TIME" "$APP_URL/api/v1/health" >"$HEALTH_RESPONSE_FILE"
  curl --fail --silent --show-error --max-time "$CURL_MAX_TIME" "$APP_URL/" >"$HOME_SNAPSHOT_FILE"
}

backup_database() {
  if [[ "$SKIP_BACKUP" == "1" ]]; then
    echo "backup skipped: SKIP_BACKUP=1" >&2
    return 0
  fi
  if [[ ! -f "$DB_PATH" ]]; then
    echo "backup skipped: $DB_PATH not present (first deploy?)" >&2
    return 0
  fi
  if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "backup ERROR: sqlite3 binary not found on host; refusing to deploy without a backup" >&2
    return 1
  fi
  mkdir -p "$BACKUP_DIR"
  local stamp out
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  out="$BACKUP_DIR/qcs-${stamp}-pre-${deployed_sha:0:12}.db"
  echo "backup_starting=$out"
  sqlite3 "$DB_PATH" ".backup '$out'"
  if ! sqlite3 "$out" "PRAGMA integrity_check;" | head -1 | grep -q '^ok$'; then
    echo "backup integrity check FAILED for $out" >&2
    return 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$out" > "$out.sha256"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$out" > "$out.sha256"
  fi
  echo "backup_complete=$out"
  if [[ -n "$BACKUP_REMOTE_DEST" ]]; then
    if command -v rsync >/dev/null 2>&1; then
      echo "backup_shipping_to=$BACKUP_REMOTE_DEST"
      if rsync -avz "$out" "$out.sha256" "$BACKUP_REMOTE_DEST/"; then
        echo "backup_shipped=$BACKUP_REMOTE_DEST"
      else
        echo "backup_ship_failed: rsync to $BACKUP_REMOTE_DEST failed" >&2
        return 1
      fi
    else
      echo "backup_ship_skipped: rsync not installed" >&2
    fi
  fi
  # Best-effort retention: prune local snapshots older than retention window.
  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'qcs-*.db' -mtime +"$BACKUP_RETENTION_DAYS" -delete 2>/dev/null || true
  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'qcs-*.db.sha256' -mtime +"$BACKUP_RETENTION_DAYS" -delete 2>/dev/null || true
}

deploy_current_release() {
  compose config >/dev/null
  compose build "${build_flags[@]}"
  # Phase 1.4 (OPS): snapshot the live database before applying migrations.
  # Migrations are not auto-rolled-back on deploy failure (see README), so
  # the pre-migration snapshot is the operator's recovery point.
  if ! backup_database; then
    echo "refusing to migrate without a successful pre-deploy backup" >&2
    return 1
  fi
  compose run --rm "$SERVICE_NAME" /app/qcs-migrate
  compose up -d --remove-orphans
  wait_for_container_health
  verify_public_endpoints
}

rollback_release() {
  if [[ "$ROLLBACK_ON_FAILURE" != "1" ]]; then
    echo "rollback_on_failure=disabled" >&2
    return 1
  fi
  if [[ -z "$previous_successful_sha" || "$previous_successful_sha" == "$deployed_sha" ]]; then
    echo "rollback skipped: no previous successful release recorded" >&2
    return 1
  fi
  echo "attempting rollback_to=$previous_successful_sha" >&2
  git checkout --detach "$previous_successful_sha"
  compose config >/dev/null
  compose build "${build_flags[@]}"
  compose up -d --remove-orphans
  wait_for_container_health
  verify_public_endpoints
  printf '%s\n' "$previous_successful_sha" > "$LAST_SUCCESSFUL_SHA_FILE"
  echo "rollback_complete=$previous_successful_sha" >&2
}

require_production_compose
if ! deploy_current_release; then
  echo "deployment_failed_sha=$deployed_sha" >&2
  if ! rollback_release; then
    echo "rollback_failed=1" >&2
    docker logs --tail 200 "$CONTAINER_NAME" >&2 || true
    exit 1
  fi
  echo "rollback restored previous release; keeping deploy job failed" >&2
  exit 1
fi

printf '%s\n' "$deployed_sha" > "$LAST_SUCCESSFUL_SHA_FILE"
echo "health_response=$(sed -n '1p' "$HEALTH_RESPONSE_FILE")"
echo "home_title=$(grep -o '<title>[^<]*</title>' "$HOME_SNAPSHOT_FILE" | sed -n '1p')"
echo "deployment_complete=$deployed_sha"
