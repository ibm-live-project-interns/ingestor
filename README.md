# Ingestor Services

Backend microservices for Sentrix. Handles API serving, data ingestion, event routing, and AI processing.

## Architecture

```
┌─────────────────┐      ┌─────────────────┐     ┌─────────────────┐
│   Datasource    │────>│  Ingestor Core  │────>│  Event Router   │
│   (External)    │      │     :8001       │     │     :8082       │
└─────────────────┘      └─────────────────┘     └────────┬────────┘
                                                          │
                         ┌────────────────────────────────┼────────────────┐
                         │                                │                │
                         v                                v                v
                  ┌─────────────┐                 ┌─────────────┐  ┌─────────────┐
                  │ AI Core     │                 │ API Gateway │  │   Kafka     │
                  │   :9000     │                 │   :8080     │  │   :9092     │
                  │  (watsonx)  │                 │  (Direct)   │  │             │
                  └─────────────┘                 └──────┬──────┘  └─────────────┘
                                                         │
                                                         │ Proxied by nginx
                                                         v
                                                  ┌─────────────┐
                                                  │ UI (nginx)  │
                                                  │   :3000     │
                                                  └─────────────┘
```

The UI at port 3000 uses nginx to proxy API requests to the API Gateway at port 8080.

## Services

### 1. API Gateway (Port 8080)

The primary backend service. REST API with JWT auth, RBAC, and 101 registered routes.

**Tech:** Go 1.24 + Gin 1.11 + GORM 1.31 + PostgreSQL 15

**Handler files** (18 files in `api_gateway/handlers/`):

| File | Description |
|------|-------------|
| `alerts.go` | Alert CRUD, summary, severity distribution, time series, acknowledge, dismiss, resolve |
| `audit.go` | Audit log listing with filters, pagination |
| `auth.go` | Login (JWT + demo mode), register, logout, Google OAuth |
| `common.go` | Dashboard summary/metrics/charts, devices, AI metrics/insights, trends, reports |
| `configuration.go` | Threshold rules, notification channels, escalation policies, maintenance windows CRUD |
| `device_groups.go` | Device group CRUD, assign/remove devices |
| `global_settings.go` | Global settings GET/PUT (maintenance mode, auto-resolve, AI correlation) |
| `health_extended.go` | Service status endpoint with real service health checks |
| `oncall.go` | On-call current, schedule (demo mode) |
| `profile.go` | Self-service profile update, password change |
| `runbooks.go` | Runbook CRUD with RBAC, 10 demo runbooks, search/category filter |
| `service_status.go` | Docker container status, container logs |
| `settings.go` | User settings CRUD |
| `sla.go` | SLA reports, violations, trend data |
| `tickets.go` | Ticket CRUD, stats, comments, delete |
| `topology.go` | Network topology nodes + edges (demo mode) |
| `users.go` | User management CRUD, reset-password (sysadmin) |
| `email_test_handler.go` | Email delivery test |

**Route groups** (from `main.go`):
- **Internal** (`/api/internal/`) - Service-to-service, API key auth: events
- **Public** (`/api/v1/`) - No auth: login, register, logout, health, Google OAuth
- **Protected** (`/api/v1/`) - JWT auth: alerts, tickets, devices, AI, trends, dashboard, reports, configuration, runbooks, device-groups, global-settings, users, profile, SLA, audit, on-call, topology, service-status, settings

### 2. Ingestor Core (Port 8001)

Central ingestion point for network events.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/ingest/event` | Receive normalized events from Datasource |
| GET | `/health` | Health check |

### 3. Event Router (Port 8082)

Routes events by severity to downstream services.

**Configuration** (`config.json`):
```json
{
  "critical": "http://ai-core:9000/events",
  "high": "http://ai-core:9000/events",
  "medium": "http://api-gateway:8080/api/internal/events",
  "low": "http://api-gateway:8080/api/internal/events",
  "info": "http://api-gateway:8080/api/internal/events"
}
```

Critical/high -> AI Core for Watson analysis. Medium/low/info -> API Gateway directly.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/route` | Route event to destination |
| GET | `/health` | Health check |

### 4. Agents API (Port 9000)

Minimal stub in `agents_api/`. The full Watson AI integration lives in the separate [ai-core](../ai-core/) service.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/events` | Process event with AI |
| GET | `/health` | Health check |

## Shared Package (`shared/`)

Common code used by all ingestor services.

```
shared/
├── config/         # GetEnv() helper
├── constants/      # Severity levels, event types
├── database/       # GORM repositories
│   ├── database.go       # DB connection, auto-migration
│   ├── alert_repo.go     # Alert CRUD, filtering, stats
│   ├── audit_repo.go     # Audit log with search/filter/stats
│   ├── config_repo.go    # Configuration tables CRUD
│   ├── ticket_repo.go    # Ticket CRUD, real MTTR computation
│   └── user_repo.go      # User CRUD, GetAll, SoftDelete
├── errors/         # Structured error types
├── httpclient/     # HTTP client utilities
├── logger/         # Structured logging
├── middleware/     # Gin middleware
│   ├── auth.go           # JWT validation
│   ├── headers.go        # Security headers
│   ├── logger.go         # Request logging
│   ├── ratelimit.go      # Rate limiting
│   └── security.go       # CORS
├── models/         # GORM models
│   ├── event.go          # Event, RoutedEvent (pipeline models)
│   ├── alert.go          # Alert (DB model)
│   ├── user.go           # User, Session (DB models)
│   ├── ticket.go         # Ticket, Comment (DB models)
│   ├── configuration.go  # ThresholdRule, NotificationChannel, EscalationPolicy, MaintenanceWindow
│   └── audit.go          # AuditLog with JSONB
├── rbac/           # Role-based access control
│   └── permissions.go    # 5 roles, 13 permissions
└── routing/        # Event routing utilities
```

## Database (PostgreSQL 15)

17 tables defined in `postgres-init/init.sql`:

| Table | Purpose |
|-------|---------|
| `users` | User accounts with roles |
| `sessions` | Active JWT sessions |
| `alerts` | Network alerts with AI analysis |
| `alert_history` | Historical alert data |
| `devices` | Network device inventory |
| `tickets` | Issue tracking |
| `ticket_comments` | Ticket comments |
| `threshold_rules` | Alert threshold configuration |
| `notification_channels` | Notification endpoints |
| `escalation_policies` | Alert escalation rules |
| `maintenance_windows` | Scheduled maintenance |
| `ingestion_data` | Raw ingested events |
| `ai_results` | AI analysis results |
| `ai_metrics` | AI performance metrics |
| `api_keys` | Service-to-service auth |
| `audit_logs` | User action audit trail |
| `runbooks` | Knowledge base articles |

## Quick Start

### Docker (Recommended)

```bash
cd infra/prod
docker compose up -d --build
```

### Local Development

```bash
# Start each service in separate terminals
cd api_gateway && go run main.go
cd ingestor_core && go run main.go
cd event_router && go run main.go
```

### Environment Variables

Copy `.env.example` to `.env` and fill in values:

```bash
cp .env.example .env
```

Key variables for the API Gateway:

| Variable | Required | Description |
|----------|----------|-------------|
| `JWT_SECRET` | **Yes** | Min 32-char random string. App exits without it. |
| `CORS_ALLOWED_ORIGINS` | Yes | Comma-separated allowed origins (include your frontend URL) |
| `FRONTEND_URL` | Yes (prod) | Base URL of the UI (for OAuth redirects and email links) |
| `POSTGRES_HOST` | No | DB host — omit to run in demo mode (no persistence) |
| `GOOGLE_CLIENT_ID` | No | Google OAuth client ID (enables Google login button) |
| `GOOGLE_CLIENT_SECRET` | No | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | No | Must match Google Cloud Console redirect URI |
| `SMTP_HOST` | No | SMTP server — omit to disable email features |
| `SMTP_USERNAME` | No | SMTP credentials |
| `SMTP_PASSWORD` | No | SMTP credentials |
| `INTERNAL_API_KEY` | No | API key for internal service-to-service calls |

**Production (HuggingFace Spaces):** `https://bionicop-sentrix-api.hf.space`

Set these in HF Spaces → Settings → Variables and Secrets:
```
JWT_SECRET=<strong random string>
CORS_ALLOWED_ORIGINS=https://ui-bionics-projects.vercel.app,http://localhost:5173
FRONTEND_URL=https://ui-bionics-projects.vercel.app
GOOGLE_CLIENT_ID=<from Google Cloud Console>
GOOGLE_CLIENT_SECRET=<from Google Cloud Console>
GOOGLE_REDIRECT_URL=https://bionicop-sentrix-api.hf.space/api/v1/auth/google/callback
POSTGRES_HOST=<your postgres host>
```

All env vars use the shared `config.GetEnv()` helper with sensible defaults.

## Authentication

**Demo mode** (no DB): Any non-empty email/password works, JWT generated in-memory. Email patterns map to roles:
- `*admin*` or `*sysadmin*` -> sysadmin
- `*sre*` -> sre
- `*netadmin*` -> network-admin
- `*senior*` -> senior-eng
- Default -> network-ops

**Production** (with DB): Validates against `users` table with bcrypt password hashing.

Default admin: `admin@admin.com` / `admin123` (role: `sysadmin`)

```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@admin.com", "password": "admin123"}'

curl http://localhost:8080/api/v1/alerts \
  -H "Authorization: Bearer <token>"
```

## Health Checks

```bash
curl http://localhost:8080/api/v1/health  # API Gateway
curl http://localhost:8001/health          # Ingestor Core
curl http://localhost:8082/health          # Event Router
curl http://localhost:9000/health          # Agents API (ai-core)
```
