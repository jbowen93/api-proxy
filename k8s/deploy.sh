#!/bin/bash

set -e

echo "🚀 Deploying API Proxy to Kubernetes..."
echo

# Build Docker images for k3s
echo "📦 Building Docker images..."
docker build -t api-key-service:latest ./api-key-service/
docker build -t backend-service:latest ./backend-service/
docker build -t billing-sidecar:latest ./billing-sidecar/

# Import images into k3s
echo "📥 Importing images into k3s..."
k3s ctr images import <(docker save api-key-service:latest)
k3s ctr images import <(docker save backend-service:latest)
k3s ctr images import <(docker save billing-sidecar:latest)

# Apply Kubernetes manifests
echo "🎯 Applying Kubernetes manifests..."
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/postgres.yaml
kubectl apply -f k8s/redis.yaml

# Wait for databases to be ready
echo "⏳ Waiting for databases to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres -n api-proxy --timeout=60s
kubectl wait --for=condition=ready pod -l app=redis -n api-proxy --timeout=60s

# Deploy services
kubectl apply -f k8s/api-key-service.yaml
kubectl apply -f k8s/backend-service.yaml
kubectl apply -f k8s/billing-sidecar.yaml
kubectl apply -f k8s/envoy.yaml

# Wait for all pods to be ready
echo "⏳ Waiting for all services to be ready..."
kubectl wait --for=condition=ready pod -l app=api-key-service -n api-proxy --timeout=60s
kubectl wait --for=condition=ready pod -l app=backend-service -n api-proxy --timeout=60s
kubectl wait --for=condition=ready pod -l app=billing-sidecar -n api-proxy --timeout=60s
kubectl wait --for=condition=ready pod -l app=envoy -n api-proxy --timeout=60s

echo
echo "✅ Deployment complete!"
echo
echo "📋 Service Status:"
kubectl get pods -n api-proxy
echo
echo "🌐 Access the API Gateway:"
echo "  Envoy Proxy: http://localhost:8000"
echo "  Envoy Admin: http://localhost:9901"
echo
echo "🔑 Test API Key Creation:"
echo "  kubectl port-forward -n api-proxy svc/api-key-service 8080:8080"
echo "  curl -X POST http://localhost:8080/api/keys -H 'Content-Type: application/json' -d '{\"user_id\": \"test-user\", \"name\": \"Test Key\"}'"
echo
echo "🌍 Test Gateway Access:"
echo "  kubectl port-forward -n api-proxy svc/envoy 8000:8000"
echo "  curl -H \"Authorization: Bearer YOUR_API_KEY\" http://localhost:8000/hello"