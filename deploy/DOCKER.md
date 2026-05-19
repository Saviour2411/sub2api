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
