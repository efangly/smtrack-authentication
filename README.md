# SMtrack Auth Service

Authentication and Authorization microservice for the **SMtrack** platform, built with Go using a clean hexagonal architecture.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Framework | [Fiber v3](https://github.com/gofiber/fiber) |
| Database | PostgreSQL (GORM, `goose` migrations) |
| Cache | Redis |
| Messaging | RabbitMQ |
| Auth | JWT (Access + Refresh tokens) |
| Container | Docker / Kubernetes |
| Language | Go 1.26 |

## Architecture

This service follows **Hexagonal Architecture** (Ports & Adapters):

```
internal/
├── core/                      # Business logic (framework-independent)
│   ├── domain/                # Entities, enums, Actor, File, *Patch types
│   ├── ports/                 # Driving (services.go) and driven ports
│   │                          #   (repositories, cache, upload, messaging, security)
│   └── services/              # Use cases, tenant scope (scope.go), audit changes
├── legacyimport/              # Copies the old hospital/ward database in (cmd/import-legacy)
└── purge/                     # Permanent removal of expired soft-deleted records (cmd/purge)
└── adapters/
    ├── driven/                # Outbound adapters (infra)
    │   ├── cache/             # Redis adapter
    │   ├── database/          # PostgreSQL repositories, patch→column mapping
    │   ├── messaging/         # RabbitMQ adapter
    │   ├── security/          # JWT issuer + bcrypt password hasher
    │   └── upload/            # File upload adapter
    └── driving/               # Inbound adapters (HTTP)
        └── http/
            ├── handlers/      # Route handlers
            ├── middleware/     # JWT auth, roles, error handling
            ├── dto/           # Request/response types
            └── router.go      # Route registration
```

## API Endpoints

All routes are prefixed with `/auth`.

### Public Routes

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/auth/register` | Register a new user (supports image upload) |
| `POST` | `/auth/login` | Login with username & password |
| `GET`  | `/auth/health` | Health check |

### Authenticated Routes (JWT required)

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/auth/refresh` | Refresh access token (refresh JWT) | — |
| `POST` | `/auth/logout` | Revoke the caller's refresh token (body: `token`) | Any authenticated |
| `PATCH` | `/auth/reset/:username` | Reset password (signs the user's other sessions out) | Any authenticated |
| `GET` | `/auth/report/access` | Access review: role, status, tenant, departments, last login | SUPER, SERVICE, ADMIN, LEGACY_ADMIN |

### User Management

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/auth/user` | Create user | SUPER, SERVICE, ADMIN, LEGACY_ADMIN |
| `GET` | `/auth/user` | List all users | SUPER, SERVICE, ADMIN, LEGACY_ADMIN |
| `GET` | `/auth/user/:id` | Get user by ID | Any authenticated |
| `PUT` | `/auth/user/:id` | Update user (non-admins: own record only) | Any authenticated |
| `DELETE` | `/auth/user/:id` | Soft delete user | SUPER, SERVICE, ADMIN, LEGACY_ADMIN |
| `POST` | `/auth/user/:id/restore` | Restore user | SUPER, SERVICE, ADMIN, LEGACY_ADMIN |

### Tenant Management

A tenant is a hospital, laboratory, factory, clinic, pharmacy or other organisation.

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/auth/tenant` | Create tenant | SUPER, SERVICE |
| `GET` | `/auth/tenant` | List visible tenants | Any authenticated |
| `GET` | `/auth/tenant/:id` | Get tenant (with label profile and departments) | Any authenticated |
| `PUT` | `/auth/tenant/:id` | Update tenant | SUPER, SERVICE |
| `PUT` | `/auth/tenant/:id/labels` | Set the tenant's own names for "tenant" and "department" per language | SUPER, SERVICE, own tenant's ADMIN / LEGACY_ADMIN |
| `DELETE` | `/auth/tenant/:id` | Soft delete (only when it has no users or departments) | SUPER |
| `POST` | `/auth/tenant/:id/restore` | Restore | SUPER |

### Department Management

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/auth/department` | Create department | SUPER, SERVICE |
| `GET` | `/auth/department` | List visible departments | Any authenticated |
| `GET` | `/auth/department/:id` | Get department by ID | Any authenticated |
| `PUT` | `/auth/department/:id` | Update department | SUPER, SERVICE |
| `DELETE` | `/auth/department/:id` | Soft delete | SUPER, SERVICE |
| `POST` | `/auth/department/:id/restore` | Restore | SUPER, SERVICE |

### Audit Log

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `GET` | `/auth/audit` | List audit entries (`entity`, `action`, `entityId`, `actorId`, `limit`, `offset`) | SUPER, SERVICE |

### Tenant scope

A user belongs to one tenant and to any number of that tenant's departments. ADMIN and
LEGACY_ADMIN are confined to their own tenant: user, tenant and department lookups by id, user
updates and deletes, and the departments a user is placed in must all belong to it. Other roles
see only their own tenant and the departments they are linked to. Hidden tenants are visible to
SUPER only. Anything outside reads as not found. SUPER and SERVICE act system-wide.

## User Roles

| Role | Description |
|------|-------------|
| `SUPER` | Full system access |
| `SERVICE` | Service-level access (inter-service) |
| `ADMIN` | Tenant admin |
| `USER` | Standard user |
| `LEGACY_ADMIN` | Legacy system admin |
| `LEGACY_USER` | Legacy system user |
| `GUEST` | Read-only guest |

## Configuration

Copy `.env.example` to `.env` and set the required values:

```env
# Server
PORT=8080
NODE_ENV=production

# Database (required)
DATABASE_URL=postgres://user:password@host:5432/dbname

# JWT (required)
JWT_SECRET=your-secret-key
JWT_REFRESH_SECRET=your-refresh-secret-key
# Both secrets: >= 32 characters and different, or the service will not start.
EXPIRE_TIME=15m
REFRESH_EXPIRE_TIME=7d
BCRYPT_COST=12          # minimum 12

# Redis
REDIS_HOST=localhost:6379
REDIS_PASSWORD=

# RabbitMQ
RABBITMQ=amqp://user:password@host:5672/

# Pictures (MinIO, encrypted with AES-256-GCM; the bucket is created at startup if missing)
S3_ENDPOINT=http://localhost:9000
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
S3_BUCKET=auth-bucket
IMAGE_ENCRYPTION_KEY=   # openssl rand -base64 32
IMAGE_BASE_URL=https://host/auth/image

# Logging
LOG_LEVEL=warn
```

## Running Locally

### Prerequisites

- Go 1.26+
- PostgreSQL
- Redis
- RabbitMQ (optional, for messaging)

### Run

```bash
go mod download
go run main.go
```

### Run Tests

```bash
go test ./...                       # unit, golden-response and architecture tests (no services needed)
# real Postgres/Redis (empty databases): see CLAUDE.md for TEST_DATABASE_URL, REDIS_TEST_ADDR
go test -tags integration ./...
go test ./internal/adapters/driving/http/handlers/ -run TestGolden -update   # after an intended response change
go test -tags=e2e ./test/e2e/... -v # end-to-end against a real stack (needs a seeded SUPER account, ~7 min); see test/e2e/main_test.go
```

The domain vocabulary is in [`GLOSSARY.md`](GLOSSARY.md), architectural decisions in
[`docs/adr/`](docs/adr/), and the refactor plan in
[`docs/refactor/hexagonal-plan.md`](docs/refactor/hexagonal-plan.md).

## Docker

### Build

```bash
docker build -t smtrack-auth-service .
```

### Run

```bash
docker run -p 8080:8080 --env-file .env smtrack-auth-service
```

## Kubernetes Deployment

The service deploys to the `smtrack` namespace with 2 replicas.

The tenant/department version runs beside the old one (ADR 0004). `k8s/deploy.yaml` is the old
version; the new one is `k8s/deploy-v2.yaml`. Secrets (JWT secrets, `DATABASE_URL`, Redis
password) come from the `authentication-v2-secrets` Secret; `k8s/secrets.example.yaml` is only a
template. Cutover and rollback steps are in [`docs/ops/cutover-runbook.md`](docs/ops/cutover-runbook.md).

```bash
kubectl apply -f k8s/deploy-v2.yaml
kubectl apply -f k8s/purge-cronjob.yaml   # weekly removal of expired soft-deleted records
# k8s/import-legacy-job.yaml is a one-off, applied only during cutover (see the runbook)
```

ConfigMaps still used for non-secret settings: `authentication-config` (MinIO endpoint and bucket, image base URL, trusted
proxies), `redis-config`, `rabbitmq-config`. The security mapping is in
[`docs/security/iso27001-controls.md`](docs/security/iso27001-controls.md).

### Health Check

```
GET /auth/health
```

Kubernetes probes (readiness & liveness) hit this endpoint every 30 seconds.

## Docker Image

```
siamatic/smtrack-auth-service:3.0.0
```
