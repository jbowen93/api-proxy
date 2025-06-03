#!/bin/bash

echo "🧹 Cleaning up API Proxy from Kubernetes..."

# Delete all resources
kubectl delete namespace api-proxy --ignore-not-found=true

# Wait for namespace deletion
echo "⏳ Waiting for namespace deletion..."
kubectl wait --for=delete namespace/api-proxy --timeout=60s

echo "✅ Cleanup complete!"