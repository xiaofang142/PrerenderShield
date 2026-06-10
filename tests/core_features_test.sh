#!/bin/bash
# ============================================================
# Prerender Shield — 核心功能脚本测试
# ============================================================
set -e

BASE="http://localhost:9598/api/v1"
SITE_SERVER="http://localhost:8082"
PASS=0
FAIL=0

green() { echo -e "\033[32m$1\033[0m"; }
red()   { echo -e "\033[31m$1\033[0m"; }
blue()  { echo -e "\033[34m$1\033[0m"; }

check() {
  local desc="$1" expected="$2" actual="$3"
  if echo "$actual" | grep -q "$expected"; then
    green "  ✅ $desc"
    PASS=$((PASS+1))
  else
    red "  ❌ $desc"
    echo "     expected: $expected"
    echo "     got:      $actual"
    FAIL=$((FAIL+1))
  fi
}

# 登录获取 token
TOKEN=$(curl -s -X POST "$BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"Test123456!"}' | \
  grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  TOKEN=$(curl -s -X POST "$BASE/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"Test123456!"}' | \
    grep -o '"token":"[^"]*"' | cut -d'"' -f4)
fi
AUTH="Authorization: Bearer $TOKEN"

blue "============================================"
blue "  核心功能 1: WAF 防火墙检测"
blue "============================================"

# 1.1 SQL注入检测 (通过站点服务器)
R=$(curl -s "http://localhost:8082/?id=1'+OR+'1'='1")
check "SQL Injection blocking" "403\|Blocked\|blocked\|Access Denied" "$R"

# 1.2 XSS检测
R=$(curl -s "http://localhost:8082/?q=<script>alert(1)</script>")
check "XSS blocking" "403\|Blocked\|blocked\|Access Denied" "$R"

# 1.3 路径遍历检测
R=$(curl -s "http://localhost:8082/?file=../../../etc/passwd")
check "Path Traversal blocking" "403\|Blocked\|blocked\|Access Denied" "$R"

blue ""
blue "============================================"
blue "  核心功能 2: 站点管理 CRUD"
blue "============================================"

# 2.1 创建站点
R=$(curl -s -X POST "$BASE/sites" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"feature-test","domain":"feature.example.com","port":8090,"mode":"proxy"}')
SITE_ID=$(echo "$R" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
check "Create site (id exists)" "id" "$R"

# 2.2 获取站点列表
R=$(curl -s "$BASE/sites" -H "$AUTH")
check "List sites" '"code":200' "$R"

# 2.3 更新站点
R=$(curl -s -X PUT "$BASE/sites/$SITE_ID" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"name":"feature-test-updated"}')
check "Update site" '"code":200' "$R"

# 2.4 删除站点
R=$(curl -s -X DELETE "$BASE/sites/$SITE_ID" -H "$AUTH")
check "Delete site" '"code":200' "$R"

blue ""
blue "============================================"
blue "  核心功能 3: 渲染预热 API"
blue "============================================"

# 3.1 预热统计
R=$(curl -s "$BASE/preheat/stats" -H "$AUTH")
check "Preheat stats" '"code":200' "$R"

# 3.2 触发预热
R=$(curl -s -X POST "$BASE/preheat/trigger" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"siteId":"site1"}')
check "Trigger preheat" '"code":200' "$R"

# 3.3 预热 URL 列表
R=$(curl -s "$BASE/preheat/urls" -H "$AUTH")
check "Preheat URLs" '"code":200' "$R"

# 3.4 爬虫头列表
R=$(curl -s "$BASE/preheat/crawler-headers" -H "$AUTH")
check "Crawler headers" '"code":200' "$R"

# 3.5 预热站点列表
R=$(curl -s "$BASE/preheat/sites" -H "$AUTH")
check "Preheat sites" '"code":200' "$R"

blue ""
blue "============================================"
blue "  核心功能 4: 搜索引擎推送 API"
blue "============================================"

R=$(curl -s "$BASE/push/stats" -H "$AUTH")
check "Push stats" '"code":200' "$R"

R=$(curl -s "$BASE/push/sites" -H "$AUTH")
check "Push sites" '"code":200' "$R"

R=$(curl -s "$BASE/push/config" -H "$AUTH")
check "Push config" '"code":200' "$R"

R=$(curl -s -X POST "$BASE/push/config" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"siteId":"site1","enabled":true}')
check "Push config update" '"code":200' "$R"

blue ""
blue "============================================"
blue "  核心功能 5: 爬虫日志 API"
blue "============================================"

R=$(curl -s "$BASE/crawler/logs" -H "$AUTH")
check "Crawler logs" '"code":200' "$R"

R=$(curl -s "$BASE/crawler/stats" -H "$AUTH")
check "Crawler stats" '"code":200' "$R"

blue ""
blue "============================================"
blue "  核心功能 6: 2FA 双因素认证"
blue "============================================"

R=$(curl -s "$BASE/2fa/status" -H "$AUTH")
check "2FA status" '"code":200' "$R"

R=$(curl -s -X POST "$BASE/2fa/enable" -H "$AUTH" -H "Content-Type: application/json" -d '{}')
check "2FA enable" '"code":200' "$R"

R=$(curl -s -X POST "$BASE/2fa/confirm" -H "$AUTH" -H "Content-Type: application/json" -d '{"code":"Test123456!"}')
check "2FA confirm" '"code":200' "$R"

R=$(curl -s -X POST "$BASE/2fa/disable" -H "$AUTH" -H "Content-Type: application/json" -d '{"code":"Test123456!"}')
check "2FA disable" '"code":200' "$R"

blue ""
blue "============================================"
blue "  核心功能 7: 监控与日志"
blue "============================================"

R=$(curl -s "$BASE/monitoring/stats" -H "$AUTH")
check "Monitoring stats" '"code":200' "$R"

R=$(curl -s "$BASE/logs" -H "$AUTH")
check "Access logs" '"code":200' "$R"

R=$(curl -s "$BASE/overview" -H "$AUTH")
check "Overview dashboard" '"code":200' "$R"

blue ""
blue "============================================"
blue "  核心功能 8: 系统管理"
blue "============================================"

R=$(curl -s "$BASE/system/config" -H "$AUTH")
check "System config" '"code":200' "$R"

R=$(curl -s -X POST "$BASE/system/config" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"server":{"api_port":9598}}')
check "System config update" '"code":200' "$R"

R=$(curl -s "$BASE/health")
check "Health check" '"code":200' "$R"

R=$(curl -s "$BASE/version")
check "Version" '"code":200' "$R"

blue ""
blue "============================================"
blue "  测试总结"
blue "============================================"
TOTAL=$((PASS+FAIL))
echo -e "  总计: $TOTAL  通过: \033[32m$PASS\033[0m  失败: \033[31m$FAIL\033[0m"
if [ "$FAIL" -eq 0 ]; then
  green "  ✅ 所有核心功能测试通过!"
else
  red "  ❌ 存在 $FAIL 个失败!"
  exit 1
fi
