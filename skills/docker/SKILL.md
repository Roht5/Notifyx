# Docker & Local Dev Skill

Use this skill when writing the `Dockerfile`, `docker-compose.yml`, or debugging local dev environment issues.

## When to use

- Phase 13 — Deployment
- Setting up local dev environment for the first time
- Debugging "works on my machine" issues
- Adding a new service to docker-compose

## Dockerfile — multi-stage build

Always use multi-stage. Final image should be minimal (no Go toolchain, no source code).

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Cache dependencies separately from source code
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o notifyx ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19
WORKDIR /app

# Security: don't run as root
RUN addgroup -S notifyx && adduser -S notifyx -G notifyx

COPY --from=builder /app/notifyx .
COPY --from=builder /app/migrations ./migrations

USER notifyx

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./notifyx"]
```

## docker-compose.yml — local dev

```yaml
version: "3.9"

services:
  app:
    build: .
    ports:
      - "8080:8080"
    env_file: .env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: on-failure

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: notifyx
      POSTGRES_PASSWORD: notifyx
      POSTGRES_DB: notifyx
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U notifyx"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./config/prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    volumes:
      - grafana_data:/var/lib/grafana
    depends_on:
      - prometheus

volumes:
  postgres_data:
  grafana_data:
```

Note: Kafka is **not** in docker-compose — use Confluent Cloud free tier even for local dev. It avoids the complexity of running Zookeeper/KRaft locally.

## .env.example

```bash
ENV=development
PORT=8080

# Database (docker-compose local)
DATABASE_URL=postgres://notifyx:notifyx@localhost:5432/notifyx?sslmode=disable

# Redis (docker-compose local)
REDIS_URL=redis://localhost:6379

# Kafka (Confluent Cloud free tier)
KAFKA_BROKERS=pkc-xxxxx.us-east-1.aws.confluent.cloud:9092
KAFKA_GROUP_ID=notifyx-consumers
KAFKA_API_KEY=your_confluent_api_key
KAFKA_API_SECRET=your_confluent_api_secret

# Channels
RESEND_API_KEY=re_xxxxxxxxxxxx
FCM_CREDENTIALS_PATH=./config/firebase-credentials.json
FAST2SMS_API_KEY=your_fast2sms_key

# Observability
OTLP_ENDPOINT=http://localhost:4317
```

## Prometheus config

```yaml
# config/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: notifyx
    static_configs:
      - targets: ["app:8080"]
    metrics_path: /metrics
```

## render.yaml — Render deployment

```yaml
services:
  - type: web
    name: notifyx
    runtime: docker
    dockerfilePath: ./Dockerfile
    plan: free
    healthCheckPath: /health
    envVars:
      - key: ENV
        value: production
      - key: DATABASE_URL
        fromDatabase:
          name: notifyx-db
          property: connectionString
      - key: REDIS_URL
        sync: false
      - key: KAFKA_BROKERS
        sync: false
      - key: KAFKA_API_KEY
        sync: false
      - key: KAFKA_API_SECRET
        sync: false
      - key: RESEND_API_KEY
        sync: false
      - key: FCM_CREDENTIALS_PATH
        sync: false
      - key: FAST2SMS_API_KEY
        sync: false

databases:
  - name: notifyx-db
    plan: free
```

## Common issues

**`go mod download` fails in Docker:**
→ Make sure `go.sum` is committed and up to date. Run `go mod tidy` locally first.

**App starts before DB is ready:**
→ Use `depends_on` with `condition: service_healthy` and add a healthcheck to postgres service (already in template above).

**Migrations don't run in container:**
→ Ensure `migrations/` folder is copied in Dockerfile: `COPY --from=builder /app/migrations ./migrations`

**Port already in use:**
→ `lsof -i :8080` to find what's using it, then kill it or change the port mapping.

**Changes not reflected after rebuild:**
→ `docker-compose up --build` forces a rebuild. `docker-compose up` reuses cached image.

## Useful commands

```bash
docker-compose up --build        # start everything, rebuild app
docker-compose up -d             # start in background
docker-compose logs -f app       # stream app logs
docker-compose down              # stop everything
docker-compose down -v           # stop + delete volumes (fresh DB)
docker exec -it <postgres_container> psql -U notifyx  # connect to DB
```
