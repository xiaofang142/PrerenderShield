#!/bin/bash

# 测试触发渲染预热功能是否正常工作

# 设置API地址
API_URL="http://localhost:9000/api/v1/preheat/trigger"

# 设置测试站点ID（根据实际情况修改）
SITE_ID="9dbfaa2b-9015-4012-a00a-8e7f47ab01dd"

# 测试触发预热
echo "测试触发渲染预热..."
curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "{\"siteId\": \"$SITE_ID\"}" \
  -v

echo "\n测试完成，请检查服务是否正常运行。"
