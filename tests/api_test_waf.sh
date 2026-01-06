#!/bin/bash

BASE_URL="http://localhost:9598/api/v1"
USERNAME="admin"
PASSWORD="123456" # Default password

# 1. Login
echo "Logging in..."
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}")
TOKEN=$(echo $LOGIN_RESP | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "Login failed: $LOGIN_RESP"
  exit 1
fi

echo "Login success. Token obtained."

# 2. Get Sites (to get a valid site ID)
echo "Getting sites..."
SITES_RESP=$(curl -s -X GET "$BASE_URL/sites" -H "Authorization: Bearer $TOKEN")
# Simple extraction of the first ID found in the JSON response
SITE_ID=$(echo $SITES_RESP | grep -o '"id":"[^"]*' | head -n 1 | cut -d'"' -f4)

if [ -z "$SITE_ID" ]; then
    echo "No sites found in response or failed to parse. Using default ID from config."
    SITE_ID="9dbfaa2b-9015-4012-a00a-8e7f47ab01dd"
fi
echo "Using Site ID: $SITE_ID"

# 3. Get WAF Config
echo "Getting WAF Config for site $SITE_ID..."
curl -s -X GET "$BASE_URL/sites/$SITE_ID/waf" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool || echo "Failed to parse JSON response"
echo ""

# 4. Update WAF Config
echo "Updating WAF Config for site $SITE_ID..."
UPDATE_RESP=$(curl -s -X PUT "$BASE_URL/sites/$SITE_ID/waf" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "action": {
        "default_action": "block",
        "block_message": "Blocked by Test Script"
    },
    "rate_limit_count": 50,
    "rate_limit_window": 60,
    "custom_block_page": "<html>Blocked</html>"
  }')
echo "Update Response: $UPDATE_RESP"
echo ""

# 5. Verify Update
echo "Verifying Update..."
curl -s -X GET "$BASE_URL/sites/$SITE_ID/waf" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool || echo "Failed to parse JSON response"
echo ""

echo "Test finished."
