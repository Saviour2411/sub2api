#!/bin/sh
set -eu

DEPLOY_DIR=${DEPLOY_DIR:-}
SUB2API_IMAGE=${SUB2API_IMAGE:-}
SUB2API_TAG=${SUB2API_TAG:-}
COMPOSE_FILE=${COMPOSE_FILE:-docker-compose.yml}
HEALTH_URL_INPUT=${HEALTH_URL:-}
HEALTH_RETRIES=${HEALTH_RETRIES:-30}
HEALTH_INTERVAL=${HEALTH_INTERVAL:-5}

if [ "$COMPOSE_FILE" != "docker-compose.yml" ]; then
    echo "生产部署只允许使用 docker-compose.yml" >&2
    exit 1
fi

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

# 在更新版本或重建容器前核验活动清单、真实挂载和现有健康状态。
if [ ! -f docker-compose.sub2api.yml ] || ! cmp -s "$COMPOSE_FILE" docker-compose.sub2api.yml; then
    echo "活动 Compose 与 docker-compose.sub2api.yml 必须完全一致" >&2
    exit 1
fi

verify_bind_mount() {
    container=$1
    source=$2
    destination=$3
    mounts=$(docker inspect "$container" --format '{{range .Mounts}}{{printf "%s\t%s\t%s\n" .Type .Source .Destination}}{{end}}')
    printf '%s\n' "$mounts"
    actual=$(printf '%s\n' "$mounts" | awk -F '\t' -v target="$destination" '$3 == target {print}')
    expected=$(printf 'bind\t%s\t%s' "$source" "$destination")
    if [ "$actual" != "$expected" ]; then
        echo "$container 的 $destination 未使用预期的 bind mount，拒绝部署" >&2
        return 1
    fi
}

docker compose -f "$COMPOSE_FILE" ps
verify_bind_mount sub2api-postgres "$DEPLOY_DIR/postgres_data" /var/lib/postgresql/data
verify_bind_mount sub2api "$DEPLOY_DIR/data" /app/data
verify_bind_mount sub2api-redis "$DEPLOY_DIR/redis_data" /data

if command -v curl >/dev/null 2>&1; then
    if ! curl -fsS --max-time 5 "$HEALTH_URL" >/dev/null; then
        echo "部署前健康检查失败，未修改版本或重建容器" >&2
        exit 1
    fi
elif command -v wget >/dev/null 2>&1; then
    if ! wget -q -T 5 -O /dev/null "$HEALTH_URL"; then
        echo "部署前健康检查失败，未修改版本或重建容器" >&2
        exit 1
    fi
else
    echo "缺少健康检查工具，拒绝部署" >&2
    exit 1
fi

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
        echo "缺少健康检查工具，不能将部署判定为成功" >&2
        docker compose -f "$COMPOSE_FILE" ps
        exit 1
    fi

    echo "Waiting for health check ${i}/${HEALTH_RETRIES}..."
    i=$((i + 1))
    sleep "$HEALTH_INTERVAL"
done

echo "Health check failed: $HEALTH_URL" >&2
docker compose -f "$COMPOSE_FILE" ps >&2 || true
docker compose -f "$COMPOSE_FILE" logs --tail=200 sub2api >&2 || true
exit 1
