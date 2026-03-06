# SMtrack Auth Service

Authentication and Authorization microservice for the **SMtrack** platform, built with Go using a clean hexagonal architecture.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Framework | [Fiber v3](https://github.com/gofiber/fiber) |
| Database | PostgreSQL (GORM) |
| Cache | Redis |
| Messaging | RabbitMQ |
| Auth | JWT (Access + Refresh tokens) |
| Container | Docker / Kubernetes |
| Language | Go 1.25 |

## Architecture

This service follows **Hexagonal Architecture** (Ports & Adapters):

```
internal/
├── core/                      # Business logic (framework-independent)
│   ├── domain/                # Domain models & enums
│   ├── ports/                 # Interface definitions
│   └── services/              # Use case implementations
└── adapters/
    ├── driven/                # Outbound adapters (infra)
    │   ├── cache/             # Redis adapter
    │   ├── database/          # PostgreSQL repositories
    │   ├── messaging/         # RabbitMQ adapter
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
| `PATCH` | `/auth/reset/:id` | Reset password | Any authenticated |

### User Management

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/auth/user` | Create user | SUPER, SERVICE, ADMIN, LEGACY_ADMIN |
| `GET` | `/auth/user` | List all users | SUPER, SERVICE, ADMIN, LEGACY_ADMIN |
| `GET` | `/auth/user/:id` | Get user by ID | Any authenticated |
| `PUT` | `/auth/user/:id` | Update user | Any authenticated |
| `DELETE` | `/auth/user/:id` | Delete user | Any authenticated |

### Hospital Management

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/auth/hospital` | Create hospital | SUPER, SERVICE |
| `GET` | `/auth/hospital` | List all hospitals | Any authenticated |
| `GET` | `/auth/hospital/:id` | Get hospital by ID | Any authenticated |
| `PUT` | `/auth/hospital/:id` | Update hospital | SUPER, SERVICE |
| `DELETE` | `/auth/hospital/:id` | Delete hospital | SUPER |

### Ward Management

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/auth/ward` | Create ward | SUPER, SERVICE |
| `GET` | `/auth/ward` | List all wards | Any authenticated |
| `GET` | `/auth/ward/:id` | Get ward by ID | Any authenticated |
| `PUT` | `/auth/ward/:id` | Update ward | Any authenticated |
| `DELETE` | `/auth/ward/:id` | Delete ward | Any authenticated |

## User Roles

| Role | Description |
|------|-------------|
| `SUPER` | Full system access |
| `SERVICE` | Service-level access (inter-service) |
| `ADMIN` | Hospital admin |
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
EXPIRE_TIME=1h
REFRESH_EXPIRE_TIME=7d

# Redis
REDIS_HOST=localhost:6379
REDIS_PASSWORD=

# RabbitMQ
RABBITMQ=amqp://user:password@host:5672/

# File Upload
UPLOAD_PATH=/path/to/upload/directory

# Logging
LOG_LEVEL=warn
```

## Running Locally

### Prerequisites

- Go 1.25+
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
go test ./...
```

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

```bash
kubectl apply -f k8s/deploy.yaml
```

The deployment uses the following ConfigMaps and Secrets:
- `authentication-config` — app settings (PORT, DATABASE_URL, token expiry, upload path)
- `jwt-secret` — JWT_SECRET, JWT_REFRESH_SECRET
- `redis-config` — Redis connection info
- `rabbitmq-config` — RabbitMQ connection URL

### Health Check

```
GET /auth/health
```

Kubernetes probes (readiness & liveness) hit this endpoint every 30 seconds.

## Docker Image

```
siamatic/smtrack-auth-service:2.0.0
```
