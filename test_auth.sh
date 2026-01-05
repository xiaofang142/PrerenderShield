#!/bin/bash

# 测试登录获取token

# 设置API地址
API_URL="http://localhost:9598/api/v1/auth/login"

# 设置登录凭证
USERNAME="admin"
PASSWORD="123456"

# 测试登录
echo "测试登录获取token..."
TOKEN=$(curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}" \
  | jq -r '.data.token')

if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
  echo "登录成功，获取到token: $TOKEN"
  echo "$TOKEN" > .token
  echo "token已保存到 .token 文件"
else
  echo "登录失败"
  exit 1
fi
