# Ingestor Services

Backend microservices for the NOC Dashboard - handles data ingestion, event routing, and AI processing.

## Architecture

```
┌─────────────────┐      ┌─────────────────┐     ┌─────────────────┐
│   Datasource    │────▶│  Ingestor Core  │────▶│  Event Router   │
│   (External)    │      │     :8001       │     │     :8082       │
└─────────────────┘      └─────────────────┘     └────────┬────────┘
                                                          │
                         ┌────────────────────────────────┼────────────────┐
                         │                                │                │
                         ▼                                ▼                ▼
                  ┌─────────────┐                 ┌─────────────┐  ┌─────────────┐
                  │ Agents API  │                 │ API Gateway │  │   Kafka     │
                  │   :9000     │                 │   :8080     │  │   :9092     │
                  │  (watsonx)  │                 │  (Direct)   │  │             │
                  └─────────────┘                 └──────┬──────┘  └─────────────┘
                                                         │
                                                         │ Proxied by nginx
                                                         ▼
                                                  ┌─────────────┐
                                                  │ UI (nginx)  │
                                                  │   :3000     │
                                                  └─────────────┘
```

**Note:** The UI at port 3000 uses nginx to proxy API requests to the API Gateway at port 8080.

## Shared Package

All services use a common `shared/` package for:
- **Models** (`shared/models/event.go`) - `Event`, `RoutedEvent` structs
- **Constants** (`shared/constants/`) - Severity levels, event types
- **Config** (`shared/config/env.go`) - `GetEnv()` helper

This eliminates code duplication across services.

## Services

### 1. API Gateway (Port 8080)

REST API serving the UI with authentication and authorization.

**Features:**
- JWT-based authentication
- CORS configuration
- Rate limiting
- Security headers

**Key Endpoints:**

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/login` | User authentication |
| GET | `/api/v1/alerts` | List all alerts |
| GET | `/api/v1/alerts/:id` | Get alert details |
| POST | `/api/v1/tickets` | Create ticket |
| POST | `/api/internal/events` | Internal API (no auth) for service-to-service |
| GET | `/api/v1/health` | Health check |

### 2. Ingestor Core (Port 8001)

Central ingestion point for all network events.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/ingest/metadata` | Receive normalized events |
| GET | `/health` | Health check |

### 3. Event Router (Port 8082)

Routes events to appropriate downstream services based on event type.

**Configuration** (`config.json`):
```json
{
  "critical": "http://api-gateway:8080/api/internal/events",
  "warning": "http://api-gateway:8080/api/internal/events",
  "info": "http://api-gateway:8080/api/internal/events"
}
```

**Note:** Uses Docker service name `api-gateway` and internal endpoint (no auth required).

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/route` | Route event to destination |
| GET | `/health` | Health check |

### 4. Agents API (Port 9000)

IBM watsonx AI integration for intelligent event analysis.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/events` | Process event with AI |
| GET | `/health` | Health check |

## Quick Start

### Run with Docker (Recommended)

The easiest way is to use docker-compose from the [ui repository](https://github.com/ibm-live-project-interns/ui):

```bash
# Clone both repos side-by-side
git clone https://github.com/ibm-live-project-interns/ui.git
git clone https://github.com/ibm-live-project-interns/ingestor.git

# Start all services
cd ui
docker compose up -d --build
```

### Run Locally

```bash
# Start each service in separate terminals
cd api_gateway && go run main.go
cd ingestor_core && go run main.go
cd event_router && go run main.go
cd agents_api && go run main.go
```

## Environment Variables

Create a `.env` file (see `.env.example`):

```bash
# API Gateway
API_GATEWAY_PORT=8080
JWT_SECRET=your-secure-secret-key
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=admin
POSTGRES_PASSWORD=secret
POSTGRES_DB=noc_alerts

# Kafka
KAFKA_BROKERS=localhost:9092
```

## API Authentication

**Demo Mode:** Accepts any non-empty username and password.

```bash
# Login (Direct API access on port 8080)
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123", "role": {"id": "admin", "text": "Administrator"}}'

# Use token
curl http://localhost:8080/api/v1/alerts \
  -H "Authorization: Bearer <your-token>"
```

**Note:** When using the web UI at `http://localhost:3000`, nginx proxies API requests from port 3000 to port 8080.

## Health Checks

```bash
curl http://localhost:8080/api/v1/health  # API Gateway
curl http://localhost:8001/health          # Ingestor Core
curl http://localhost:8082/health          # Event Router
curl http://localhost:9000/health          # Agents API
```

## Documentation

Full documentation is in the [docs repository](https://github.com/ibm-live-project-interns/docs):

- [API Reference](https://github.com/ibm-live-project-interns/docs/blob/main/docs/API.md) - Complete REST API docs
- [Architecture](https://github.com/ibm-live-project-interns/docs/blob/main/docs/ARCHITECTURE.md) - System design
- [Environment Config](https://github.com/ibm-live-project-interns/docs/blob/main/docs/ENVIRONMENT.md) - All variables
- [Deployment](https://github.com/ibm-live-project-interns/docs/blob/main/docs/DEPLOYMENT.md) - Deployment guide

**Offline Access:** If you have all repos cloned side-by-side, docs are at `../docs/docs/`

## Related Repositories

| Repository | Description |
|------------|-------------|
| [docs](https://github.com/ibm-live-project-interns/docs) | Documentation |
| [ui](https://github.com/ibm-live-project-interns/ui) | Frontend dashboard |
| [datasource](https://github.com/ibm-live-project-interns/datasource) | Data simulation |
| [infra](https://github.com/ibm-live-project-interns/infra) | Infrastructure |
