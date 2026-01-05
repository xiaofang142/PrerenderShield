#!/bin/bash

# 测试清除缓存功能是否正常工作

# 设置API地址（使用正确的端口）
API_URL="http://localhost:9598/api/v1/preheat/clear-cache"

# 设置测试站点ID（根据实际情况修改）
SITE_ID="9dbfaa2b-9015-4012-a00a-8e7f47ab01dd"

# 测试清除缓存
echo "测试清除缓存..."
curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "{\"siteId\": \"$SITE_ID\"}" \
  -v

echo "\n测试完成，请检查服务是否正常运行。"
