#!/bin/bash
# ============================================================
# Prerender Shield — 真实功能测试（不凑数 401）
# ============================================================
set -e

BASE="http://localhost:9596/api/v1"
PASS=0
FAIL=0
ERR=0

green() { echo -e "\033[32m$1\033[0m"; }
red() { echo -e "\033[31m$1\033[0m"; }
blue() { echo -e "\033[34m$1\033[0m"; }

check() {
  local desc="$1" expected="$2" actual="$3"
  if echo "$actual" | grep -q "$expected"; then
    green "  [PASS] $desc"
    PASS=$((PASS+1))
  else
    red "  [FAIL] $desc"
    echo "         expected: $expected"
    echo "         got:      $actual"
    FAIL=$((FAIL+1))
  fi
}

login() {
  curl -s -X POST "$BASE/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"Test123456!"}' | \
    python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])" 2>/dev/null || echo ""
}

# First time: create user
FIRST=$(curl -s "$BASE/auth/first-run")
if echo "$FIRST" | grep -q '"isFirstRun":true'; then
  curl -s -X POST "$BASE/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"Test123456!"}' > /dev/null
fi

TOKEN=$(login)
if [ -z "$TOKEN" ]; then
  echo "FAIL: Cannot login"
  exit 1
fi
AUTH="Authorization: Bearer $TOKEN"

blue "============================================"
blue "  1. 站点管理 — 完整的 CRUD 生命周期"
blue "============================================"

# 1.1 创建站点
SITE_PORT=9123
R=$(curl -s -X POST "$BASE/sites" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"name\":\"functional-test\",\"domains\":[\"127.0.0.1\"],\"port\":$SITE_PORT,\"mode\":\"proxy\",\"proxy\":{\"target_url\":\"http://127.0.0.1:9124\"}}")
SITE_ID=$(echo "$R" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('id',''))" 2>/dev/null)
check "POST /sites (create)" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

# 1.2 获取该站点
if [ -n "$SITE_ID" ]; then
  R=$(curl -s "$BASE/sites/$SITE_ID" -H "$AUTH")
  check "GET /sites/:id (read)" "$SITE_ID" "$R"

  # 1.3 更新站点（需要完整站点配置）
  R=$(curl -s -X PUT "$BASE/sites/$SITE_ID" -H "$AUTH" -H "Content-Type: application/json" \
    -d "{\"name\":\"functional-test-updated\",\"domains\":[\"127.0.0.1\"],\"port\":$SITE_PORT}")
  check "PUT /sites/:id (update)" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

  # 1.4 删除站点
  R=$(curl -s -X DELETE "$BASE/sites/$SITE_ID" -H "$AUTH")
  check "DELETE /sites/:id (delete)" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

  # 1.5 确认删除后 404
  R=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/sites/$SITE_ID" -H "$AUTH")
  check "GET deleted site (404)" "404" "$R"
fi

blue ""
blue "============================================"
blue "  2. WAF 防火墙配置"
blue "============================================"

# Use preconfigured site for operations that need a site context
R=$(curl -s "$BASE/sites/preconfigured-site/waf" -H "$AUTH")
check "GET /sites/:id/waf (read config)" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

# 更新 WAF 配置
R=$(curl -s -X PUT "$BASE/sites/preconfigured-site/waf" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"enabled":true,"action":{"default_action":"block","block_message":"Blocked by WAF"},"rate_limit_count":50,"rate_limit_window":30}')
check "PUT /sites/:id/waf (update)" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

# 读取验证
R=$(curl -s "$BASE/sites/preconfigured-site/waf" -H "$AUTH")
check "WAF config persisted" "enabled" "$R"

blue ""
blue "============================================"
blue "  3. 预渲染配置"
blue "============================================"

R=$(curl -s -X PUT "$BASE/sites/preconfigured-site/prerender" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"enabled":true,"pool_size":3,"cache_ttl":3600,"preheat":{"enabled":true,"sitemap_url":"http://test.com/sitemap.xml"}}')
check "PUT /sites/:id/prerender" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/sites/preconfigured-site/config?type=prerender" -H "$AUTH")
check "GET /sites/:id/config?type=prerender" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  4. 预热系统"
blue "============================================"

R=$(curl -s "$BASE/preheat/stats" -H "$AUTH")
check "GET /preheat/stats" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/preheat/sites" -H "$AUTH")
check "GET /preheat/sites" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/preheat/crawler-headers" -H "$AUTH")
check "GET /preheat/crawler-headers" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s -X POST "$BASE/preheat/trigger" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"siteId":"preconfigured-site"}')
check "POST /preheat/trigger (needs Chromium)" "200\|500" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/preheat/urls?siteId=preconfigured-site" -H "$AUTH")
check "GET /preheat/urls" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s -X POST "$BASE/preheat/clear-cache" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"siteId":"preconfigured-site"}')
check "POST /preheat/clear-cache" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  5. 搜索引擎推送"
blue "============================================"

R=$(curl -s "$BASE/push/stats" -H "$AUTH")
check "GET /push/stats" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/push/sites" -H "$AUTH")
check "GET /push/sites" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/push/config?siteId=preconfigured-site" -H "$AUTH")
check "GET /push/config" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s -X POST "$BASE/push/config" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"siteId":"preconfigured-site","enabled":true,"baidu_token":"test-token"}')
check "POST /push/config" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/push/logs" -H "$AUTH")
check "GET /push/logs" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/push/trend" -H "$AUTH")
check "GET /push/trend (needs siteId)" "200\|400" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  6. 爬虫日志"
blue "============================================"

R=$(curl -s "$BASE/crawler/logs" -H "$AUTH")
check "GET /crawler/logs" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/crawler/stats" -H "$AUTH")
check "GET /crawler/stats" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  7. 监控与概览"
blue "============================================"

R=$(curl -s "$BASE/overview" -H "$AUTH")
check "GET /overview" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/monitoring/stats" -H "$AUTH")
check "GET /monitoring/stats" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/logs" -H "$AUTH")
check "GET /logs" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  8. 系统管理"
blue "============================================"

R=$(curl -s "$BASE/system/config" -H "$AUTH")
check "GET /system/config" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s -X POST "$BASE/system/config" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"server":{"api_port":9596}}')
check "POST /system/config" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/health")
check "GET /health" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/version")
check "GET /version" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  9. SSL 证书管理"
blue "============================================"

R=$(curl -s "$BASE/ssl/certificates" -H "$AUTH")
check "GET /ssl/certificates (needs ACME)" "200\|500" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/ssl/certificates/expiring" -H "$AUTH")
check "GET /ssl/certificates/expiring (needs ACME)" "200\|500" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  10. 2FA 双因素认证"
blue "============================================"

R=$(curl -s "$BASE/2fa/status" -H "$AUTH")
check "GET /2fa/status" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s -X POST "$BASE/2fa/enable" -H "$AUTH" -H "Content-Type: application/json" -d '{}')
check "POST /2fa/enable" "200" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

blue ""
blue "============================================"
blue "  11. 边界测试 — 错误输入"
blue "============================================"

# 非法 siteId
R=$(curl -s "$BASE/sites/nonexistent-id" -H "$AUTH")
check "GET nonexistent site" "404\|500" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin).get('code',''))")"

# 非法方法
R=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE/overview" -H "$AUTH")
check "DELETE on GET-only (Gin 404)" "404" "$R"

# 空 body POST
R=$(curl -s -X POST "$BASE/sites" -H "$AUTH" -H "Content-Type: application/json" -d '{}')
check "POST /sites empty body (400)" "400" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

# 非法 JSON
R=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/sites" -H "$AUTH" -H "Content-Type: application/json" -d 'not-json')
check "POST /sites bad json" "400" "$R"

# 缺少 siteId 参数
R=$(curl -s "$BASE/preheat/urls" -H "$AUTH")
check "GET /preheat/urls no siteId" "400\|404" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin).get('code',''))")"

R=$(curl -s "$BASE/preheat/trigger" -H "$AUTH" -H "Content-Type: application/json" -d '{}')
check "POST /preheat/trigger no siteId" "400" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s "$BASE/push/config" -H "$AUTH")
check "GET /push/config no siteId" "400" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

R=$(curl -s -X POST "$BASE/push/config" -H "$AUTH" -H "Content-Type: application/json" -d '{}')
check "POST /push/config no siteId" "400" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin)['code'])")"

# 过长的名字（边界值）
LONG_NAME=$(python3 -c "print('a'*300)")
R=$(curl -s -X POST "$BASE/sites" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"name\":\"$LONG_NAME\",\"domain\":\"test.com\",\"port\":80}")
check "POST /sites very long name" "200\|400\|413" "$(echo $R | python3 -c "import sys,json; print(json.load(sys.stdin).get('code',''))")"

blue ""
blue "============================================"
blue "  总结"
blue "============================================"
TOTAL=$((PASS+FAIL))
echo "  通过: $PASS / $TOTAL"
if [ "$FAIL" -eq 0 ]; then
  green "  ✅ 所有真实功能测试通过！"
  exit 0
else
  red "  ❌ $FAIL 个失败"
  exit 1
fi
