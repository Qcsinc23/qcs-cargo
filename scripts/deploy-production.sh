#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/qcs-cargo}"
TARGET_BRANCH="${TARGET_BRANCH:-main}"
DEPLOY_REF="${DEPLOY_REF:-origin/$TARGET_BRANCH}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
SERVICE_NAME="${SERVICE_NAME:-qcs-cargo}"
CONTAINER_NAME="${CONTAINER_NAME:-qcs_cargo}"
APP_URL="${APP_URL:-https://qcs-cargo.com}"
LOCK_FILE="${LOCK_FILE:-/tmp/qcs-cargo-production-deploy.lock}"
HEALTH_RETRIES="${HEALTH_RETRIES:-20}"
HEALTH_SLEEP_SECONDS="${HEALTH_SLEEP_SECONDS:-3}"
BUILD_FLAGS="${BUILD_FLAGS:---pull}"

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

deployed_sha="$(git rev-parse HEAD)"
echo "deploying_sha=$deployed_sha"

docker compose -f "$COMPOSE_FILE" build $BUILD_FLAGS
docker compose -f "$COMPOSE_FILE" run --rm "$SERVICE_NAME" /app/qcs-migrate
docker compose -f "$COMPOSE_FILE" up -d --remove-orphans

health_status=""
for _ in $(seq 1 "$HEALTH_RETRIES"); do
  health_status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$CONTAINER_NAME" 2>/dev/null || true)"
  if [[ "$health_status" == "healthy" ]]; then
    break
  fi
  if [[ "$health_status" == "dead" || "$health_status" == "exited" ]]; then
    docker logs --tail 200 "$CONTAINER_NAME" >&2 || true
    echo "container failed with state=$health_status" >&2
    exit 1
  fi
  sleep "$HEALTH_SLEEP_SECONDS"
done

if [[ "$health_status" != "healthy" ]]; then
  docker compose -f "$COMPOSE_FILE" ps >&2 || true
  docker logs --tail 200 "$CONTAINER_NAME" >&2 || true
  echo "container did not become healthy" >&2
  exit 1
fi

curl --fail --silent --show-error "$APP_URL/api/v1/health" >/tmp/qcs-deploy-health.json
curl --fail --silent --show-error "$APP_URL/" >/tmp/qcs-deploy-home.html

echo "health_response=$(sed -n '1p' /tmp/qcs-deploy-health.json)"
echo "home_title=$(grep -o '<title>[^<]*</title>' /tmp/qcs-deploy-home.html | sed -n '1p')"
echo "deployment_complete=$deployed_sha"
