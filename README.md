# Notifyx — Multi-Tenant Distributed Notification Service

Notifyx is a high-throughput, multi-tenant distributed notification service built in **Go**. It manages Email, Push, SMS, and In-app (WebSocket) channels via an asynchronous, event-driven architecture using **Kafka**, **PostgreSQL**, and **Redis**.

Notifyx is built with production-grade reliability guarantees, including sliding-window rate limiting, atomic idempotency deduplication, worker-level exponential backoff retries, and a Dead Letter Queue (DLQ).

---

## 🏗️ System Architecture

The core pipeline is split into a **synchronous ingestion API** (handles authorization, validation, rate-limiting, and deduplication) and an **asynchronous delivery loop** (processes message queues and executes provider requests).

```
                            [ HTTP CLIENTS ]
                                   │
                                   ▼
                   [ Ingestion API (Echo Web Server) ]
                                   │
             ┌─────────────────────┼─────────────────────┐
             ▼                     ▼                     ▼
     [ API Key Auth ]       [ Rate Limiting ]      [ Deduplication ]
     (Hashed SHA-256)      (Redis Sliding ZSET)   (Redis Atomic SetNX)
             │                     │                     │
             └─────────────────────┼─────────────────────┘
                                   ▼
                       [ PostgreSQL DB Persistence ]
                       (Notification: Pending Status)
                                   │
                                   ▼
                       [ Kafka Producer Publish ]
                                   │
     ┌──────────────────────┬──────┴───────────────┬──────────────────────┐
     ▼                      ▼                      ▼                      ▼
notifyx.email          notifyx.sms            notifyx.push          notifyx.inapp
     │                      │                      │                      │
     ▼                      ▼                      ▼                      ▼
[Email Consumer]      [SMS Consumer]        [Push Consumer]       [In-App Consumer]
(Retry Loop 3x)       (Retry Loop 3x)       (Retry Loop 3x)       (Gorilla WebSocket)
     │                      │                      │                      │
     └──────────────────────┼──────────────────────┘                      │
                            ▼ (Failed after 3 attempts)                   ▼
                      [ notifyx.dlq Topic ]                             [ Redis ]
                            │                                     (Offline Queues)
                            ▼
                     [ DLQ Consumer ]
                            │
                            ▼
                    [ Postgres Audit ]
                     (dlq_messages)
```

---

## 🚀 Key Features (Phases 1–5)

* **Multi-Tenant Separation:** All entities (notifications, keys, configurations) are partition-isolated by `tenant_id` at the database and memory layers.
* **Deterministic API Key Auth:** Incoming client requests are authenticated using `X-API-Key` headers matched against one-way `SHA-256` database hashes.
* **Sliding Window Rate Limiter:** Evaluates both channel-specific rates (`max_per_min`) and tenant-wide global rates (`global_rate_cap`) in a single round-trip Lua script utilizing Redis Sorted Sets (ZSETs).
* **Rate-Limit Queuing & Replayer:** Exceeded limits automatically defer notifications to `queued_rate_limited` status. A background replayer polls, re-checks quotas, and dispatches them to Kafka once space clears.
* **Atomic Deduplication:** A Redis-backed idempotency check (`dedup:{SHA-256(key)}`) handles concurrent requests and returns the original notification details without writing duplicate rows.
* **Reliable Consumer Loop:** Decoupled workers read channel topics, coordinate provider sends, and commit offsets only after delivery completes. Workers retry failed deliveries up to 3x with backoff delays (1s, 2s, 4s) before routing to the DLQ.

---

## 🛠️ Technology Stack

* **Language:** Go 1.26 (clean architecture structure)
* **Web Framework:** Echo v4 (routing, logging, request tracking)
* **Message Broker:** Kafka (Confluent Cloud compatible / segmentio wire client)
* **Primary Database:** PostgreSQL (connection pooling via `pgx/v5`, versioned migrations)
* **Cache & Memory Store:** Redis (sliding window logs, dedup key-values, presence)
* **Structured Logger:** Uber Zap (JSON output in production, color-coded console in dev)

---

## 📂 Codebase Directory Structure

```
notifyx/
├── backend/                       # Go service directory
│   ├── cmd/server/                # Entrypoint (main.go initialization & server start)
│   ├── config/                    # Environment variable parser & validator
│   ├── migrations/                # Versioned SQL database schema scripts
│   ├── pkg/                       # Shared platform packages
│   │   └── logger/                # Uber Zap logger wrapper
│   ├── internal/                  # Core private business logic
│   │   ├── domain/                # Agnostic entity domain models (Notification, Tenant)
│   │   ├── api/                   # Routing, Middlewares, and HTTP Handlers
│   │   ├── repository/            # Database gateways (PostgreSQL & Redis)
│   │   ├── kafka/                 # Event loop producer & consumer workers
│   │   ├── ratelimit/             # Sliding-window Redis limiter & replayer
│   │   └── dedup/                 # Idempotency deduplicator
├── Review/                        # Architectural phase audits & reviews
└── requirements/                  # Design specification documents
```

---

## ⚙️ Configuration & Environment

Configuration is loaded from environment variables or a local `.env` file at startup. See [backend/.env.example](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/.env.example) for defaults:

| Variable | Description | Default |
|---|---|---|
| `PORT` | HTTP server execution port | `8080` |
| `APP_ENV` | Environment mode (`development` or `production`) | `development` |
| `LOG_LEVEL` | Log verbosity override (`debug`, `info`, `warn`, `error`) | `info` |
| `DATABASE_URL` | PostgreSQL connection string | *Required* |
| `DB_MAX_CONNS` | Postgres connection pool cap | `10` |
| `DB_MIN_CONNS` | Postgres connection pool idle floor | `0` |
| `REDIS_URL` | Redis connection URL | *Optional (disables rate limit/dedup if empty)* |
| `KAFKA_BOOTSTRAP_SERVERS` | Kafka cluster bootstrap host:port | *Optional (disables event worker if empty)* |

---

## 🏃 Local Development Setup

### 1. Database Migrations
Notifyx runs versioned database migrations automatically on startup using [golang-migrate](https://github.com/golang-migrate/migrate). Ensure your PostgreSQL database is running and configured via `DATABASE_URL`.

### 2. Launch the Backend Server
Navigate to the `backend/` directory and compile/run the application:
```bash
cd backend
go run cmd/server/main.go
```

The server will validate configurations, run database migrations, start the background rate-limit replayer, initialize Kafka consumers, and bind to your configured port (e.g. `http://localhost:8080`).

---

## 📖 API Reference (Core Ingestion)

All client-facing notification routes require authentication via the `X-API-Key` header.

### 1. Tenant Creation
Before sending notifications, register a tenant to get a client ID and private authentication key.

* **Request:** `POST /api/v1/tenants`
  ```bash
  curl -X POST http://localhost:8080/api/v1/tenants \
    -H "Content-Type: application/json" \
    -d '{"name": "Acme Corp"}'
  ```
* **Response (`201 Created`):**
  ```json
  {
    "id": "2d8fb61d-3844-4f05-88fb-c1a1795c73f3",
    "name": "Acme Corp",
    "api_key": "nfx_9e7e224e77cb81dfa010d93f77cb01..."
  }
  ```
  *(Note: The raw `api_key` is only returned once upon creation; its SHA-256 hash is stored on the server.)*

### 2. Enable Tenant Channels
Configure opt-ins for delivery channels (e.g. email, push, SMS):

* **Request:** `PUT /api/v1/tenants/:id/channels`
  ```bash
  curl -X PUT http://localhost:8080/api/v1/tenants/2d8fb61d-3844-4f05-88fb-c1a1795c73f3/channels \
    -H "Content-Type: application/json" \
    -d '{"channels": [{"channel": "email", "enabled": true}]}'
  ```

### 3. Configure Rate Limits
Set the maximum delivery throughput bounds for a tenant:

* **Request:** `PUT /api/v1/tenants/:id/rate-limits`
  ```bash
  curl -X PUT http://localhost:8080/api/v1/tenants/2d8fb61d-3844-4f05-88fb-c1a1795c73f3/rate-limits \
    -H "Content-Type: application/json" \
    -d '{
      "global_cap": 500,
      "rate_limits": [
        {"channel": "email", "max_per_min": 50}
      ]
    }'
  ```

### 4. Send a Notification
Send an immediate notification through an enabled channel.

* **Request:** `POST /api/v1/notifications/send`
  ```bash
  curl -X POST http://localhost:8080/api/v1/notifications/send \
    -H "X-API-Key: YOUR_API_KEY_HERE" \
    -H "Content-Type: application/json" \
    -d '{
      "channel": "email",
      "recipient_email": "user@example.com",
      "subject": "System Alert",
      "body": "Your instance disk usage is at 92%.",
      "priority": "high",
      "idempotency_key": "tx_unique_id_1001"
    }'
  ```
* **Response (`201 Created` or `200 OK` on duplicate):**
  ```json
  {
    "notification_id": "a98a0022-dfb8-4c91-bca4-d8906a20ffc1",
    "status": "queued"
  }
  ```

### 5. Send a Batch
Distribute a notification to multiple recipients in a single call:

* **Request:** `POST /api/v1/notifications/batch`
  ```bash
  curl -X POST http://localhost:8080/api/v1/notifications/batch \
    -H "X-API-Key: YOUR_API_KEY_HERE" \
    -H "Content-Type: application/json" \
    -d '{
      "channel": "email",
      "subject": "Product Launch",
      "body": "We are live!",
      "recipients": [
        {"recipient_email": "alpha@example.com"},
        {"recipient_email": "beta@example.com"}
      ]
    }'
  ```
* **Response (`201 Created`):**
  ```json
  {
    "results": [
      {"index": 0, "notification_id": "89fa88cc-929a-4c22-bcf8-e9f0ca90fe12"},
      {"index": 1, "notification_id": "01fbcc10-449e-4c02-a120-1a1a79fbb009"}
    ]
  }
  ```

### 6. View Notification History
Query the historical log page for a tenant, utilizing filtering query parameters:

* **Request:** `GET /api/v1/notifications/history?channel=email&page=1&limit=20`
  ```bash
  curl -H "X-API-Key: YOUR_API_KEY_HERE" \
    http://localhost:8080/api/v1/notifications/history?channel=email
  ```
