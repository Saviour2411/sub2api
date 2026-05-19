#!/bin/sh
set -eu

DEPLOY_DIR=${DEPLOY_DIR:-}
SUB2API_IMAGE=${SUB2API_IMAGE:-}
SUB2API_TAG=${SUB2API_TAG:-}
COMPOSE_FILE=${COMPOSE_FILE:-docker-compose.yml}
HEALTH_URL_INPUT=${HEALTH_URL:-}
HEALTH_RETRIES=${HEALTH_RETRIES:-30}
HEALTH_INTERVAL=${HEALTH_INTERVAL:-5}

if [ -z "$DEPLOY_DIR" ]; then
    echo "DEPLOY_DIR is required" >&2
    exit 1
fi

if [ -z "$SUB2API_IMAGE" ]; then
    echo "SUB2API_IMAGE is required" >&2
    exit 1
fi

if [ -z "$SUB2API_TAG" ]; then
    echo "SUB2API_TAG is required" >&2
    exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "docker is not installed" >&2
    exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
    echo "docker compose plugin is not available" >&2
    exit 1
fi

if [ ! -d "$DEPLOY_DIR" ]; then
    echo "$DEPLOY_DIR does not exist" >&2
    exit 1
fi

cd "$DEPLOY_DIR"

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "$COMPOSE_FILE not found in $DEPLOY_DIR" >&2
    exit 1
fi

if [ ! -f .env ]; then
    echo ".env not found in $DEPLOY_DIR" >&2
    exit 1
fi

SERVER_PORT_VALUE=${SERVER_PORT:-}
if [ -z "$SERVER_PORT_VALUE" ]; then
    SERVER_PORT_VALUE=$(grep '^SERVER_PORT=' .env 2>/dev/null | tail -n 1 | cut -d= -f2- || true)
fi
SERVER_PORT_VALUE=${SERVER_PORT_VALUE:-8080}
HEALTH_URL=${HEALTH_URL_INPUT:-http://127.0.0.1:${SERVER_PORT_VALUE}/health}

update_env_value() {
    key=$1
    value=$2
    file=.env

    escaped_value=$(printf '%s' "$value" | sed 's/[&\\]/\\&/g')

    if grep -q "^${key}=" "$file"; then
        sed -i "s|^${key}=.*|${key}=${escaped_value}|" "$file"
    else
        printf '\n%s=%s\n' "$key" "$value" >> "$file"
    fi
}

PREVIOUS_TAG=$(grep '^SUB2API_TAG=' .env 2>/dev/null | tail -n 1 | cut -d= -f2- || true)

update_env_value SUB2API_IMAGE "$SUB2API_IMAGE"
update_env_value SUB2API_TAG "$SUB2API_TAG"

echo "Deploying ${SUB2API_IMAGE}:${SUB2API_TAG} in ${DEPLOY_DIR}"
if [ -n "$PREVIOUS_TAG" ] && [ "$PREVIOUS_TAG" != "$SUB2API_TAG" ]; then
    echo "Previous SUB2API_TAG: $PREVIOUS_TAG"
fi

docker compose -f "$COMPOSE_FILE" pull sub2api
docker compose -f "$COMPOSE_FILE" up -d --no-build sub2api

i=1
while [ "$i" -le "$HEALTH_RETRIES" ]; do
    if command -v curl >/dev/null 2>&1; then
        if curl -fsS --max-time 5 "$HEALTH_URL" >/dev/null; then
            echo "Health check passed: $HEALTH_URL"
            docker compose -f "$COMPOSE_FILE" ps
            exit 0
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -q -T 5 -O /dev/null "$HEALTH_URL"; then
            echo "Health check passed: $HEALTH_URL"
            docker compose -f "$COMPOSE_FILE" ps
            exit 0
        fi
    else
        echo "Neither curl nor wget is available for health check" >&2
        docker compose -f "$COMPOSE_FILE" ps
        exit 0
    fi

    echo "Waiting for health check ${i}/${HEALTH_RETRIES}..."
    i=$((i + 1))
    sleep "$HEALTH_INTERVAL"
done

echo "Health check failed: $HEALTH_URL" >&2
docker compose -f "$COMPOSE_FILE" ps >&2 || true
docker compose -f "$COMPOSE_FILE" logs --tail=200 sub2api >&2 || true
exit 1
