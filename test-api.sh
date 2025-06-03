#!/bin/bash

echo "=== API Key Service Test ==="
echo

echo "1. Creating API Key..."
RESPONSE=$(curl -s -X POST http://localhost:8080/api/keys \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test-user", "name": "Demo API Key", "rate_limit_per_minute": 100, "rate_limit_per_day": 10000}')

API_KEY=$(echo $RESPONSE | grep -o '"plain_key":"[^"]*"' | cut -d'"' -f4)
echo "Created API Key: $API_KEY"
echo

echo "2. Testing without API Key (should be forbidden)..."
curl -s -o /dev/null -w "Status: %{http_code}\n" http://localhost:8000/test
echo

echo "3. Testing with valid API Key (should reach backend)..."
curl -s -o /dev/null -w "Status: %{http_code}\n" -H "Authorization: Bearer $API_KEY" http://localhost:8000/test
echo

echo "4. Testing with invalid API Key (should be forbidden)..."
curl -s -o /dev/null -w "Status: %{http_code}\n" -H "Authorization: Bearer invalid-key" http://localhost:8000/test
echo

echo "5. Listing API Keys for user..."
curl -s http://localhost:8080/api/keys/test-user | jq .
echo

echo "=== Test Complete ==="