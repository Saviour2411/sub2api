# Sub2API Docker Image

Sub2API is an AI API Gateway Platform for distributing and managing AI product subscription API quotas.

## Quick Start

```bash
docker run -d \
  --name sub2api \
  -p 8080:8080 \
  -e AUTO_SETUP=true \
  -e DATABASE_HOST="postgres" \
  -e DATABASE_PASSWORD="change_this_secure_password" \
  -e REDIS_HOST="redis" \
  DOCKERHUB_USERNAME/sub2api:latest
```

## Docker Compose

```yaml
version: '3.8'

services:
  sub2api:
    image: "${SUB2API_IMAGE:-DOCKERHUB_USERNAME/sub2api}:${SUB2API_TAG:-latest}"
    ports:
      - "8080:8080"
    environment:
      - AUTO_SETUP=true
      - DATABASE_HOST=db
      - DATABASE_PORT=5432
      - DATABASE_USER=postgres
      - DATABASE_PASSWORD=${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}
      - DATABASE_DBNAME=sub2api
      - DATABASE_SSLMODE=disable
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    depends_on:
      - db
      - redis

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=sub2api
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

## Startup and Database Recovery

Sub2API runs database migrations while starting. PostgreSQL may still be
recovering briefly after a host or Docker daemon restart. The application
retries transient PostgreSQL startup and connection errors with bounded
exponential backoff, then continues startup when the database is ready.
Permanent errors such as invalid credentials, migration checksum mismatches,
SQL errors, and incompatible data fail immediately.

The Compose deployment also checks PostgreSQL readiness with both `pg_isready`
and a simple SQL query. `depends_on: condition: service_healthy` helps order a
fresh Compose start, but application-level retries are still required when
Docker restores existing containers after a host restart.

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `AUTO_SETUP` | Enable Docker auto setup | Recommended | `true` |
| `DATABASE_HOST` | PostgreSQL host | Yes | - |
| `DATABASE_PASSWORD` | PostgreSQL password | Yes | - |
| `REDIS_HOST` | Redis host | Yes | - |
| `SERVER_PORT` | Server port inside container | No | `8080` |
| `SERVER_MODE` | Gin framework mode (`debug`/`release`) | No | `release` |
| `JWT_SECRET` | Fixed JWT secret for persistent sessions | Recommended | auto-generated |
| `TOTP_ENCRYPTION_KEY` | Fixed encryption key for 2FA secrets | Recommended | auto-generated |

## Supported Architectures

- `linux/amd64`
- `linux/arm64`

## Tags

- `latest` - Latest stable release
- `x.y.z` - Specific version
- `x.y` - Latest patch of minor version
- `x` - Latest minor of major version

## Links

- [GitHub Repository](https://github.com/Saviour2411/sub2api)
- [Documentation](https://github.com/Saviour2411/sub2api#readme)
