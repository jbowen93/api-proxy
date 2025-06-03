# API Key Service with Envoy Proxy

A complete API gateway solution with API key authentication, rate limiting, and usage-based billing.

## Architecture

- **Envoy Proxy**: Reverse proxy and API gateway
- **API Key Service**: Go service for API key management and authentication
- **Backend Service**: Hello world service for testing
- **Billing Sidecar**: Processes Envoy access logs for usage tracking and Stripe billing
- **PostgreSQL**: Database for API keys, usage, and billing records
- **Redis**: Rate limiting and caching

## Deployment Options

### Option 1: Docker Compose (Recommended for Development)

1. Copy environment variables:
```bash
cp .env.example .env
# Edit .env with your Stripe API key
```

2. Start services:
```bash
docker-compose up -d
```

3. Test the deployment:
```bash
./test-api.sh
```

### Option 2: Kubernetes (Recommended for Production)

#### Prerequisites
- Docker Desktop with Kubernetes enabled OR k3s cluster
- kubectl configured to access your cluster

#### Quick Deploy to Kubernetes

1. **Deploy everything:**
```bash
./k8s/deploy.sh
```

2. **Test the deployment:**
```bash
./k8s/test.sh
```

3. **Clean up when done:**
```bash
./k8s/cleanup.sh
```

#### Manual Kubernetes Deployment

1. **Build and import Docker images:**
```bash
# Build images
docker build -t api-key-service:latest ./api-key-service/
docker build -t backend-service:latest ./backend-service/
docker build -t billing-sidecar:latest ./billing-sidecar/

# For k3s - import images
k3s ctr images import <(docker save api-key-service:latest)
k3s ctr images import <(docker save backend-service:latest)
k3s ctr images import <(docker save billing-sidecar:latest)

# For Docker Desktop - images are automatically available
```

2. **Deploy to Kubernetes:**
```bash
# Create namespace and basic resources
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml

# Deploy databases first
kubectl apply -f k8s/postgres.yaml
kubectl apply -f k8s/redis.yaml

# Wait for databases to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n api-proxy --timeout=60s
kubectl wait --for=condition=ready pod -l app=redis -n api-proxy --timeout=60s

# Deploy application services
kubectl apply -f k8s/api-key-service.yaml
kubectl apply -f k8s/backend-service.yaml
kubectl apply -f k8s/billing-sidecar.yaml
kubectl apply -f k8s/envoy.yaml
```

3. **Access the services:**
```bash
# Port forward to access locally
kubectl port-forward -n api-proxy svc/envoy 8000:8000
kubectl port-forward -n api-proxy svc/api-key-service 8080:8080

# Or use LoadBalancer IP (if supported)
kubectl get svc -n api-proxy envoy
```

## API Usage

### API Key Management
- `POST /api/keys` - Create new API key
- `GET /api/keys/:user_id` - List user's API keys  
- `DELETE /api/keys/:id` - Delete API key

### Gateway Endpoints
- All requests through `http://localhost:8000` are authenticated and metered
- `GET /hello` - Hello world endpoint (shows user context)
- `ANY /echo` - Echo endpoint (shows all request headers)
- `GET /health` - Health check endpoint

### Example Usage

1. **Create an API key:**
```bash
curl -X POST http://localhost:8080/api/keys \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "name": "My API Key",
    "rate_limit_per_minute": 100,
    "rate_limit_per_day": 10000
  }'
```

2. **Test authenticated access:**
```bash
# Should work - returns backend response
curl -H "Authorization: Bearer sk-your-api-key" \
  http://localhost:8000/hello

# Should fail - returns 403 Forbidden
curl http://localhost:8000/hello
```

3. **List API keys:**
```bash
curl http://localhost:8080/api/keys/user123
```

## Features

- ✅ API key authentication via Envoy ext_authz
- ✅ Rate limiting (per minute/day) with Redis
- ✅ Usage tracking and billing with Stripe Meter API
- ✅ Access log processing via Envoy ALS
- ✅ PostgreSQL storage for keys and usage data
- ✅ Docker Compose and Kubernetes orchestration
- ✅ Hello world backend for testing
- ✅ Automated deployment and testing scripts

## Development

### Building Images Locally

```bash
# Build all services
docker build -t api-key-service:latest ./api-key-service/
docker build -t backend-service:latest ./backend-service/
docker build -t billing-sidecar:latest ./billing-sidecar/
```

### Testing

```bash
# Docker Compose
./test-api.sh

# Kubernetes
./k8s/test.sh
```

## Configuration

### Environment Variables
- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string  
- `STRIPE_API_KEY`: Stripe API key for billing (optional)

### Production Configuration
For production deployments, consider:
- TLS termination at Envoy
- Database connection pooling
- Redis clustering for high availability
- Horizontal pod autoscaling
- Resource limits and requests
- Monitoring and alerting
- Stripe webhook handling
- Persistent volume configuration