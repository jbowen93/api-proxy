# API Key Service with Envoy Proxy

A complete API gateway solution with API key authentication, rate limiting, and usage-based billing.

## Architecture

- **Envoy Proxy**: Reverse proxy and API gateway
- **API Key Service**: Go service for API key management and authentication
- **Billing Sidecar**: Processes Envoy access logs for usage tracking and Stripe billing
- **PostgreSQL**: Database for API keys, usage, and billing records
- **Redis**: Rate limiting and caching

## Quick Start

1. Copy environment variables:
```bash
cp .env.example .env
# Edit .env with your Stripe API key
```

2. Start services:
```bash
docker-compose up -d
```

3. Create an API key:
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

4. Test API access through Envoy:
```bash
curl -H "Authorization: Bearer sk-your-api-key" \
  http://localhost:8000/your-api-endpoint
```

## API Endpoints

### API Key Management
- `POST /api/keys` - Create new API key
- `GET /api/keys/:user_id` - List user's API keys  
- `DELETE /api/keys/:id` - Delete API key

### Gateway
- All requests through `http://localhost:8000` are authenticated and metered

## Features

- ✅ API key authentication via Envoy ext_authz
- ✅ Rate limiting (per minute/day) with Redis
- ✅ Usage tracking and billing with Stripe Meter API
- ✅ Access log processing via Envoy ALS
- ✅ PostgreSQL storage for keys and usage data
- ✅ Docker Compose orchestration

## Configuration

Backend service defaults to `host.docker.internal:3000`. Update `envoy/envoy.yaml` to point to your actual backend service.

For production, configure:
- Database connection pooling
- Redis clustering  
- Envoy TLS termination
- Stripe webhook handling
- Monitoring and alerting