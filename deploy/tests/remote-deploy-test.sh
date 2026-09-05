#!/bin/sh
set -eu
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
mkdir -p "$repo_root/output"
tmp_dir=$(mktemp -d "$repo_root/output/remote-deploy-test.XXXXXX")
cleanup() {
    # 仅删除当前测试在仓库内创建、且已核验绝对路径的目录。
    case "$tmp_dir" in
        "$repo_root"/output/remote-deploy-test.*) rm -rf -- "$tmp_dir" ;;
        *) echo "测试目录越界，拒绝清理" >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM
mkdir -p "$tmp_dir/bin"
cat > "$tmp_dir/bin/docker" <<'MOCK'
#!/bin/sh
set -eu
printf 'docker %s\n' "$*" >> "$MOCK_LOG"
if [ "$1" = compose ]; then exit 0; fi
if [ "$1" != inspect ]; then exit 1; fi
container=$2
type=bind
case "$container" in
    sub2api-postgres) source="$MOCK_DEPLOY_DIR/postgres_data"; destination=/var/lib/postgresql/data ;;
    sub2api) source="$MOCK_DEPLOY_DIR/data"; destination=/app/data ;;
    sub2api-redis) source="$MOCK_DEPLOY_DIR/redis_data"; destination=/data ;;
    *) exit 1 ;;
esac
case "$MOCK_CASE:$container" in
    named-postgres:sub2api-postgres) type=volume; source=/var/lib/docker/volumes/deploy_postgres_data/_data ;;
    wrong-postgres:sub2api-postgres) source="$MOCK_DEPLOY_DIR/other_postgres" ;;
    wrong-app:sub2api) source="$MOCK_DEPLOY_DIR/other_data" ;;
    wrong-redis:sub2api-redis) type=volume; source=/var/lib/docker/volumes/deploy_redis_data/_data ;;
esac
printf '%s\t%s\t%s\n' "$type" "$source" "$destination"
MOCK
cat > "$tmp_dir/bin/curl" <<'MOCK'
#!/bin/sh
set -eu
tag=$(sed -n 's/^SUB2API_TAG=//p' "$MOCK_DEPLOY_DIR/.env")
printf 'health:%s\n' "$tag" >> "$MOCK_LOG"
case "$MOCK_CASE" in
    unhealthy-before) exit 1 ;;
    unhealthy-after) [ "$tag" = old ] || exit 1 ;;
esac
exit 0
MOCK
chmod +x "$tmp_dir/bin/docker" "$tmp_dir/bin/curl"
# 生产为Linux；macOS CI夹具只适配BSD sed的原位编辑参数。
real_sed=$(command -v sed)
cat > "$tmp_dir/bin/sed" <<'MOCK'
#!/bin/sh
set -eu
if [ "$1" = -i ] && [ "$(uname -s)" = Darwin ]; then
    shift
    exec "$MOCK_REAL_SED" -i '' "$@"
fi
exec "$MOCK_REAL_SED" "$@"
MOCK
chmod +x "$tmp_dir/bin/sed"
count=0
run_case() {
    name=$1
    want=$2
    compose=docker-compose.yml
    if [ "$#" -gt 2 ]; then compose=$3; fi
    dir="$tmp_dir/$name"
    mkdir -p "$dir"
    printf 'services: {}\n' > "$dir/docker-compose.yml"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.sub2api.yml"
    printf 'SUB2API_IMAGE=old/image\nSUB2API_TAG=old\nSERVER_PORT=18080\nKEEP_ME=unchanged\n' > "$dir/.env"
    cp "$dir/.env" "$dir/env.before"
    : > "$dir/commands.log"
    case "$name" in
        different-compose) printf '# 不同文件\n' >> "$dir/docker-compose.sub2api.yml" ;;
        missing-companion) rm "$dir/docker-compose.sub2api.yml" ;;
    esac
    set +e
    PATH="$tmp_dir/bin:$PATH" MOCK_LOG="$dir/commands.log" MOCK_CASE="$name" MOCK_DEPLOY_DIR="$dir" MOCK_REAL_SED="$real_sed" \
        DEPLOY_DIR="$dir" SUB2API_IMAGE=test/sub2api SUB2API_TAG=new COMPOSE_FILE="$compose" \
        HEALTH_RETRIES=1 HEALTH_INTERVAL=0 \
        /bin/sh "$repo_root/deploy/remote-deploy.sh" > "$dir/result.log" 2>&1
    status=$?
    set -e
    if [ "$want" = success ]; then
        [ "$status" -eq 0 ] || { cat "$dir/result.log"; exit 1; }
        grep -q '^SUB2API_TAG=new$' "$dir/.env"
        grep -q '^SUB2API_IMAGE=test/sub2api$' "$dir/.env"
        grep -q '^KEEP_ME=unchanged$' "$dir/.env"
        grep -q '^health:old$' "$dir/commands.log"
        grep -q '^health:new$' "$dir/commands.log"
        grep -q '^docker compose -f docker-compose.yml up -d --no-build sub2api$' "$dir/commands.log"
        first_health=$(grep -n '^health:old$' "$dir/commands.log" | cut -d: -f1)
        first_pull=$(grep -n '^docker compose -f docker-compose.yml pull sub2api$' "$dir/commands.log" | cut -d: -f1)
        [ "$first_health" -lt "$first_pull" ]
    else
        [ "$status" -ne 0 ] || { echo "$name 未按预期拒绝" >&2; exit 1; }
        if [ "$want" = reject-before ]; then
            cmp -s "$dir/.env" "$dir/env.before"
            if grep -Eq '^docker compose .* (pull|up) ' "$dir/commands.log"; then
                echo "$name 在拒绝前执行了容器变更" >&2
                exit 1
            fi
        else
            grep -q '^SUB2API_TAG=new$' "$dir/.env"
            grep -q '^health:new$' "$dir/commands.log"
        fi
    fi
    count=$((count + 1))
    printf '通过：%s\n' "$name"
}
run_case healthy success
run_case local-compose reject-before docker-compose.local.yml
run_case standalone-compose reject-before docker-compose.standalone.yml
run_case dev-compose reject-before docker-compose.dev.yml
run_case different-compose reject-before
run_case missing-companion reject-before
run_case named-postgres reject-before
run_case wrong-postgres reject-before
run_case wrong-app reject-before
run_case wrong-redis reject-before
run_case unhealthy-before reject-before
run_case unhealthy-after reject-after
printf '远程部署安全门禁：%s项测试通过；未调用真实Docker、网络或生产配置。\n' "$count"
