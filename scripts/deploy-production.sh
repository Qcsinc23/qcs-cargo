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

deploy_current_release() {
  compose config >/dev/null
  compose build "${build_flags[@]}"
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
