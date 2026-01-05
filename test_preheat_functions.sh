#!/bin/bash

# 综合测试脚本：测试清除缓存和触发预热功能

# 设置API地址
BASE_URL="http://localhost:9598/api/v1/preheat"

# 读取token
TOKEN=$(cat .token)
if [ -z "$TOKEN" ]; then
  echo "未找到token，请先运行 ./test_auth.sh 获取token"
  exit 1
fi

# 设置测试站点ID（根据实际情况修改）
SITE_ID="9dbfaa2b-9015-4012-a00a-8e7f47ab01dd"

# 测试清除缓存
echo "\n=== 测试清除缓存功能 ==="
curl -X POST "$BASE_URL/clear-cache" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"siteId\": \"$SITE_ID\"}"

# 等待1秒
sleep 1

# 测试触发预热
echo "\n=== 测试触发预热功能 ==="
curl -X POST "$BASE_URL/trigger" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"siteId\": \"$SITE_ID\"}"

# 测试获取预热状态
echo "\n=== 测试获取预热状态 ==="
curl -X GET "$BASE_URL/stats?siteId=$SITE_ID" \
  -H "Authorization: Bearer $TOKEN"

echo "\n=== 所有测试完成 ==="
# 检查服务是否仍在运行
echo "检查服务状态..."
if pgrep -f "go run cmd/api/main.go" > /dev/null; then
  echo "✅ 服务运行正常"
else
  echo "❌ 服务已崩溃"
fi
