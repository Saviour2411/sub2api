#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

check_compose_default() {
  compose_file=$1
  key=$2
  value=$3
  expected=$(printf '      - %s=${%s:-%s}' "$key" "$key" "$value")
  expected_count=$(grep -Fxc "$expected" "$compose_file" || true)
  key_count=$(grep -Ec "^[[:space:]]*-[[:space:]]*${key}=" "$compose_file" || true)

  if [ "$expected_count" -ne 1 ] || [ "$key_count" -ne 1 ]; then
    printf '%s 必须恰好一次传入 %s，且默认值必须为 %s\n' "$compose_file" "$key" "$value" >&2
    exit 1
  fi
}

check_common_defaults() {
  compose_file=$1
  while IFS=' ' read -r key value; do
    check_compose_default "$compose_file" "$key" "$value"
  done <<'EOF'
GATEWAY_FORCE_CODEX_CLI false
GATEWAY_OPENAI_COMPACT_MODEL gpt-5.4
GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT 0
GATEWAY_OPENAI_WS_FORCE_HTTP false
GATEWAY_OPENAI_HTTP2_ENABLED true
GATEWAY_OPENAI_HTTP2_ALLOW_PROXY_FALLBACK_TO_HTTP1 true
GATEWAY_OPENAI_HTTP2_FALLBACK_ERROR_THRESHOLD 2
GATEWAY_OPENAI_HTTP2_FALLBACK_WINDOW_SECONDS 60
GATEWAY_OPENAI_HTTP2_FALLBACK_TTL_SECONDS 600
GATEWAY_OPENAI_PROXY_STREAM_CIRCUIT_FAILURE_THRESHOLD 2
GATEWAY_OPENAI_PROXY_STREAM_CIRCUIT_WINDOW_SECONDS 60
GATEWAY_OPENAI_PROXY_STREAM_CIRCUIT_TTL_SECONDS 600
GATEWAY_MAX_BODY_SIZE 268435456
GATEWAY_SCHEDULING_STICKY_SESSION_MAX_WAITING 3
GATEWAY_SCHEDULING_STICKY_SESSION_WAIT_TIMEOUT 120s
GATEWAY_SCHEDULING_FALLBACK_WAIT_TIMEOUT 30s
GATEWAY_SCHEDULING_FALLBACK_MAX_WAITING 100
GATEWAY_SCHEDULING_LOAD_BATCH_ENABLED true
GATEWAY_SCHEDULING_SLOT_CLEANUP_INTERVAL 30s
GATEWAY_SCHEDULING_DB_FALLBACK_ENABLED true
GATEWAY_SCHEDULING_DB_FALLBACK_TIMEOUT_SECONDS 0
GATEWAY_SCHEDULING_DB_FALLBACK_MAX_QPS 0
GATEWAY_SCHEDULING_OUTBOX_POLL_INTERVAL_SECONDS 1
GATEWAY_SCHEDULING_OUTBOX_LAG_WARN_SECONDS 5
GATEWAY_SCHEDULING_OUTBOX_LAG_REBUILD_SECONDS 10
GATEWAY_SCHEDULING_OUTBOX_LAG_REBUILD_FAILURES 3
GATEWAY_SCHEDULING_OUTBOX_BACKLOG_REBUILD_ROWS 10000
GATEWAY_SCHEDULING_FULL_REBUILD_INTERVAL_SECONDS 300
GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT 900
GATEWAY_IMAGE_STREAM_KEEPALIVE_INTERVAL 10
GATEWAY_IMAGE_NONSTREAM_KEEPALIVE_INTERVAL 0
GATEWAY_IMAGE_CONCURRENCY_ENABLED false
GATEWAY_IMAGE_CONCURRENCY_MAX_CONCURRENT_REQUESTS 0
GATEWAY_IMAGE_CONCURRENCY_OVERFLOW_MODE reject
GATEWAY_IMAGE_CONCURRENCY_WAIT_TIMEOUT_SECONDS 30
GATEWAY_IMAGE_CONCURRENCY_MAX_WAITING_REQUESTS 100
EOF
}

check_env_value() {
  key=$1
  value=$2
  expected_count=$(grep -Fxc "$key=$value" deploy/.env.example || true)
  key_count=$(grep -Ec "^${key}=" deploy/.env.example || true)

  if [ "$expected_count" -ne 1 ] || [ "$key_count" -ne 1 ]; then
    printf 'deploy/.env.example 必须恰好一次定义 %s=%s\n' "$key" "$value" >&2
    exit 1
  fi
}

check_line_once() {
  compose_file=$1
  expected=$2
  count=$(grep -Fxc "$expected" "$compose_file" || true)
  if [ "$count" -ne 1 ]; then
    printf '%s 必须恰好包含一次：%s\n' "$compose_file" "$expected" >&2
    exit 1
  fi
}

# 通用部署文件使用后端默认连接参数。
for compose_file in \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  check_common_defaults "$compose_file"
  check_compose_default "$compose_file" GATEWAY_MAX_CONNS_PER_HOST 1024
  check_compose_default "$compose_file" GATEWAY_MAX_IDLE_CONNS 2560
  check_compose_default "$compose_file" GATEWAY_MAX_IDLE_CONNS_PER_HOST 120
done

# 生产文件保留实例专用连接池和串行扣费保护参数。
for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.sub2api.yml
do
  check_common_defaults "$compose_file"
  check_compose_default "$compose_file" GATEWAY_MAX_CONNS_PER_HOST 2048
  check_compose_default "$compose_file" GATEWAY_MAX_IDLE_CONNS 2048
  check_compose_default "$compose_file" GATEWAY_MAX_IDLE_CONNS_PER_HOST 256
  check_compose_default "$compose_file" GATEWAY_USAGE_RECORD_WORKER_COUNT 64
  check_compose_default "$compose_file" GATEWAY_USAGE_RECORD_QUEUE_SIZE 16384
  check_compose_default "$compose_file" GATEWAY_USAGE_RECORD_TASK_TIMEOUT_SECONDS 5
  check_compose_default "$compose_file" GATEWAY_USAGE_RECORD_AUTO_SCALE_ENABLED false

  check_line_once "$compose_file" '      - "${BIND_HOST:-127.0.0.1}:${SERVER_PORT:-18080}:8080"'
  check_line_once "$compose_file" '      - ./data:/app/data'
  check_line_once "$compose_file" '      - ./postgres_data:/var/lib/postgresql/data'
  check_line_once "$compose_file" '      - ./redis_data:/data'
done

# 高容量示例与生产实例值用途不同，分别锁定，避免同步时相互覆盖。
check_env_value GATEWAY_OPENAI_WS_FORCE_HTTP false
check_env_value GATEWAY_MAX_CONNS_PER_HOST 2048
check_env_value GATEWAY_MAX_IDLE_CONNS 8192
check_env_value GATEWAY_MAX_IDLE_CONNS_PER_HOST 4096
check_env_value GATEWAY_USAGE_RECORD_WORKER_COUNT 128
check_env_value GATEWAY_USAGE_RECORD_QUEUE_SIZE 16384
check_env_value GATEWAY_USAGE_RECORD_TASK_TIMEOUT_SECONDS 5
check_env_value GATEWAY_USAGE_RECORD_AUTO_SCALE_ENABLED true

if ! cmp -s deploy/docker-compose.yml deploy/docker-compose.sub2api.yml; then
  printf '两份生产 Compose 文件必须逐字节一致\n' >&2
  exit 1
fi

printf 'Docker Compose Gateway 环境变量检查通过\n'
