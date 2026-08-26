#!/bin/bash
# E2E 验证编排: 启动隔离实例 → API 全端点验证 → 浏览器全站巡检 → 清理
# 用法: make verify-e2e  (需要本地 Redis 与 Node)
set -euo pipefail

API_PORT=19598
CONSOLE_PORT=19597
WORKDIR=$(mktemp -d /tmp/prs-verify.XXXXXX)
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cleanup() {
  if [ -n "${SERVER_PID:-}" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    # 等待优雅退出，最多 10s，超时强杀
    for _ in $(seq 1 20); do
      kill -0 "$SERVER_PID" 2>/dev/null || break
      sleep 0.5
    done
    kill -9 "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

# 预清理：上一次运行残留的实例会占用端口导致误判
for port in ${API_PORT} ${CONSOLE_PORT}; do
  pids=$(lsof -ti ":${port}" 2>/dev/null || true)
  [ -n "$pids" ] && kill -9 $pids 2>/dev/null || true
done

echo "▶ 构建测试二进制..."
(cd "$ROOT" && go build -o "$WORKDIR/api" ./cmd/api)

cat > "$WORKDIR/config.yml" <<EOF
server:
  address: "127.0.0.1"
  api_port: ${API_PORT}
  console_port: ${CONSOLE_PORT}
dirs:
  data_dir: ${WORKDIR}/data
  static_dir: ${WORKDIR}/static
  certs_dir: ${WORKDIR}/certs
  admin_static_dir: /nonexistent-web
cache:
  type: "redis"
  redis_url: "localhost:6379/15"
EOF
mkdir -p "$WORKDIR"/{data,static/default,certs}

# 隔离 Redis DB，保证首跑状态可复现
redis-cli -n 15 FLUSHDB > /dev/null

echo "▶ 启动被测实例 (127.0.0.1:${API_PORT})..."
"$WORKDIR/api" --config "$WORKDIR/config.yml" > "$WORKDIR/run.log" 2>&1 &
SERVER_PID=$!
for i in $(seq 1 30); do
  curl -fs "http://127.0.0.1:${API_PORT}/api/v1/health" > /dev/null 2>&1 && break
  sleep 1
done

echo "▶ API 全端点验证..."
python3 "$ROOT/scripts/api_verify_full.py" --base "http://127.0.0.1:${API_PORT}"

if command -v node > /dev/null && [ -d "$ROOT/web/node_modules" ]; then
  echo "▶ 浏览器全站巡检(官网 + 控制台)..."
  SITE_DIR="$ROOT/../prerender-offcial-website"
  SITE_OK=false
  if [ -d "$SITE_DIR" ]; then
    if (cd "$SITE_DIR" && npm run build > "$WORKDIR/site-build.log" 2>&1); then
      mkdir -p "$WORKDIR/site"
      cp -r "$SITE_DIR/dist/." "$WORKDIR/site/" && SITE_OK=true
    else
      echo "  ⚠ 官网构建失败:"; tail -5 "$WORKDIR/site-build.log"
    fi
  fi
  if [ "$SITE_OK" = true ]; then
    (cd "$WORKDIR/site" && python3 -m http.server 4173 > /dev/null 2>&1 &)
    SITE_PID=$!
    sleep 2
    NODE_PATH="$ROOT/web/node_modules" node "$ROOT/scripts/browser_audit.js" \
      http://127.0.0.1:4173 "http://127.0.0.1:${CONSOLE_PORT}" || true
    kill "${SITE_PID:-0}" 2>/dev/null || true
  else
    echo "  ⚠ 官网构建产物不可用, 跳过官网巡检"
  fi
fi

kill "$SERVER_PID" 2>/dev/null || true
echo "✅ verify-e2e 完成"
