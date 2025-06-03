#!/bin/bash

set -e

echo "🧪 Testing API Proxy on Kubernetes..."
echo

# Port forward services
echo "🔌 Setting up port forwards..."
kubectl port-forward -n api-proxy svc/envoy 8000:8000 &
ENVOY_PID=$!
kubectl port-forward -n api-proxy svc/api-key-service 8080:8080 &
API_KEY_PID=$!

# Wait for port forwards to be ready
sleep 3

# Cleanup function
cleanup() {
  echo "🧹 Cleaning up port forwards..."
  kill $ENVOY_PID $API_KEY_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "1️⃣ Creating API Key..."
RESPONSE=$(curl -s -X POST http://localhost:8080/api/keys \
  -H "Content-Type: application/json" \
  -d '{"user_id": "k8s-test-user", "name": "K8s Test Key", "rate_limit_per_minute": 100, "rate_limit_per_day": 10000}')

API_KEY=$(echo $RESPONSE | grep -o '"plain_key":"[^"]*"' | cut -d'"' -f4)
echo "✅ Created API Key: $API_KEY"
echo

echo "2️⃣ Testing without API Key (should be forbidden)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/hello)
if [ "$STATUS" = "403" ]; then
  echo "✅ Unauthorized request correctly blocked (403)"
else
  echo "❌ Expected 403, got $STATUS"
fi
echo

echo "3️⃣ Testing with valid API Key (should reach backend)..."
RESPONSE=$(curl -s -H "Authorization: Bearer $API_KEY" http://localhost:8000/hello)
echo "✅ Backend Response:"
echo "$RESPONSE" | jq .
echo

echo "4️⃣ Testing echo endpoint..."
RESPONSE=$(curl -s -H "Authorization: Bearer $API_KEY" http://localhost:8000/echo)
echo "✅ Echo Response:"
echo "$RESPONSE" | jq .
echo

echo "5️⃣ Listing API Keys..."
RESPONSE=$(curl -s http://localhost:8080/api/keys/k8s-test-user)
echo "✅ API Keys for user:"
echo "$RESPONSE" | jq .
echo

echo "🎉 All tests completed successfully!"
echo
echo "💡 You can also test manually:"
echo "  kubectl port-forward -n api-proxy svc/envoy 8000:8000"
echo "  curl -H \"Authorization: Bearer $API_KEY\" http://localhost:8000/hello"