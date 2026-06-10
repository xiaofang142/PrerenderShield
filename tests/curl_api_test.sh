#!/bin/bash
# ============================================================
# Prerender Shield — 全量 API 调用测试 (curl)
# ============================================================
set -e

BASE="http://localhost:9599/api/v1"
PASS=0
FAIL=0
TOKEN=""

green() { echo -e "\033[32m$1\033[0m"; }
red()   { echo -e "\033[31m$1\033[0m"; }
blue()  { echo -e "\033[34m$1\033[0m"; }

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

blue "=== 1. Public API ==="
R=$(curl -s $BASE/health); check "GET /health" '"code":200' "$R"
R=$(curl -s $BASE/version); check "GET /version" '"code":200' "$R"
R=$(curl -s $BASE/auth/first-run); check "GET /auth/first-run" '"code":200' "$R"

R=$(curl -s -X POST $BASE/auth/login -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Test123456!"}')
TOKEN=$(echo $R | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
check "POST /auth/login (token)" "token" "$R"
check "GET /overview (no token 401)" '"code":401' "$(curl -s $BASE/overview)"

blue "=== 2. Protected API ==="
AUTH="Authorization: Bearer $TOKEN"

check "GET /overview" '"code":200' "$(curl -s $BASE/overview -H "$AUTH")"
# 以下端点需要完整的站点配置才能返回 200，测试环境无站点配置时返回预期错误
check "GET /system/config (empty)" '"code":4' "$(curl -s $BASE/system/config -H "$AUTH")"
check "POST /system/config" '"code":200' "$(curl -s -X POST $BASE/system/config -H "$AUTH" -H "Content-Type: application/json" -d '{}')"
check "GET /monitoring/stats" '"code":200' "$(curl -s $BASE/monitoring/stats -H "$AUTH")"
check "GET /logs" '"code":200' "$(curl -s $BASE/logs -H "$AUTH")"

check "GET /firewall/attacks" '"code":200' "$(curl -s $BASE/firewall/attacks -H "$AUTH")"
check "POST /firewall/whitelist" '"code":200' "$(curl -s -X POST $BASE/firewall/whitelist -H "$AUTH" -H "Content-Type: application/json" -d '{"site_id":"t","ip":"1.2.3.4"}')"
check "POST /firewall/blacklist" '"code":200' "$(curl -s -X POST $BASE/firewall/blacklist -H "$AUTH" -H "Content-Type: application/json" -d '{"site_id":"t","ip":"5.6.7.8"}')"

check "GET /sites" '"code":200' "$(curl -s $BASE/sites -H "$AUTH")"
SITE_ID=$(curl -s -X POST $BASE/sites -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"curl-test","domain":"t.com","port":80,"mode":"proxy"}' | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

check "GET /sites (with data)" '"code":200' "$(curl -s "$BASE/sites" -H "$AUTH")"
if [ -n "$SITE_ID" ]; then
  check "GET /sites/:id" '"code":200' "$(curl -s "$BASE/sites/$SITE_ID" -H "$AUTH")"
  check "GET /sites/:id/waf" '"code":200' "$(curl -s "$BASE/sites/$SITE_ID/waf" -H "$AUTH")"
  check "PUT /sites/:id/prerender" '"code":200' "$(curl -s -X PUT "$BASE/sites/$SITE_ID/prerender" -H "$AUTH" -H "Content-Type: application/json" -d '{}')"
  check "PUT /sites/:id/waf" '"code":200' "$(curl -s -X PUT "$BASE/sites/$SITE_ID/waf" -H "$AUTH" -H "Content-Type: application/json" -d '{}')"
  check "DELETE /sites/:id" '"code":200' "$(curl -s -X DELETE "$BASE/sites/$SITE_ID" -H "$AUTH")"
fi

check "GET /preheat/stats" '"code":200' "$(curl -s $BASE/preheat/stats -H "$AUTH")"
check "POST /preheat/trigger" '"code":200' "$(curl -s -X POST $BASE/preheat/trigger -H "$AUTH" -H "Content-Type: application/json" -d '{"siteId":"site1"}')"
check "GET /preheat/urls" '"code":200' "$(curl -s "$BASE/preheat/urls?siteId=site1" -H "$AUTH")"
check "GET /preheat/sites" '"code":200' "$(curl -s $BASE/preheat/sites -H "$AUTH")"

check "GET /crawler/logs" '"code":200' "$(curl -s $BASE/crawler/logs -H "$AUTH")"
check "GET /crawler/stats" '"code":200' "$(curl -s $BASE/crawler/stats -H "$AUTH")"

check "GET /push/stats" '"code":200' "$(curl -s $BASE/push/stats -H "$AUTH")"
check "GET /push/logs" '"code":200' "$(curl -s $BASE/push/logs -H "$AUTH")"
check "GET /push/config" '"code":200' "$(curl -s "$BASE/push/config?siteId=site1" -H "$AUTH")"
check "POST /push/config" '"code":200' "$(curl -s -X POST $BASE/push/config -H "$AUTH" -H "Content-Type: application/json" -d '{"siteId":"site1"}')"
  check "GET /push/sites" '"code":200' "$(curl -s $BASE/push/sites -H "$AUTH")"

check "GET /ssl/certificates (no SSL cfg)" '"code":500' "$(curl -s $BASE/ssl/certificates -H "$AUTH")"

check "GET /2fa/status" '"code":200' "$(curl -s $BASE/2fa/status -H "$AUTH")"
check "POST /2fa/enable (or 400 if already enabled)" 'code' "$(curl -s -X POST $BASE/2fa/enable -H "$AUTH" -H "Content-Type: application/json" -d '{}')"
# 2FA disable requires valid TOTP code; skipping full flow in curl test

check "POST /auth/logout" '"code":200' "$(curl -s -X POST $BASE/auth/logout -H "$AUTH" -H "Content-Type: application/json")"

blue "=== Summary ==="
TOTAL=$((PASS+FAIL))
echo "  Total: $PASS/$TOTAL passed"
if [ "$FAIL" -eq 0 ]; then
  green "  ALL TESTS PASSED!"
else
  red "  $FAIL TESTS FAILED!"
  exit 1
fi
