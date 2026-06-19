# Notifyx — Detailed Task Breakdown

## Phase 1 — Foundation

### Go Project Setup
- Initialize Go module (`github.com/rohit-bagade/notifyx`)
- Create full folder structure per architecture.md
- Add `.env.example` with all required env vars
- Wire `config/config.go` to read all env vars with fallbacks

### Logger
- Set up Uber Zap in `pkg/logger/`
- Support `development` (pretty) and `production` (JSON) modes
- Expose `Info`, `Error`, `Fatal`, `Warn`, `Debug` methods

### Database
- PostgreSQL connection pool in `internal/repository/postgres/`
- Migration runner (use `golang-migrate`)
- Single `001_initial_schema.sql` (up/down) covering all 9 tables — see decisions.md for why this isn't one-file-per-table

### Domain Models
- `internal/domain/notification.go` — Notification, NotificationDelivery, DLQMessage, Channel, Status, Priority enums
- `internal/domain/tenant.go` — Tenant, TenantChannel, TenantRateLimit, APIKey
- `internal/domain/template.go` — NotificationTemplate

---

## Phase 2 — Tenant & Auth

### Repository
- `TenantRepository`: Create, GetByID, GetAll, Update, Delete
- `APIKeyRepository`: Create, GetByTenantID, GetByKeyHash, Delete
- `TenantChannelRepository`: Upsert, GetByTenantID
- `TenantRateLimitRepository`: Upsert, GetByTenantID

### API
- `POST /api/v1/tenants` — create tenant + generate API key
- `GET /api/v1/tenants` — list all tenants (super-admin)
- `GET /api/v1/tenants/:id` — get tenant
- `PUT /api/v1/tenants/:id` — update tenant name
- `DELETE /api/v1/tenants/:id` — delete tenant
- `PUT /api/v1/tenants/:id/channels` — update channel opt-ins
- `PUT /api/v1/tenants/:id/rate-limits` — update rate limits

### Middleware
- API key auth middleware: extract key from `X-API-Key` header, hash, look up in DB, attach tenant to context
- Request logging middleware (Zap)
- Request ID middleware

---

## Phase 3 — Kafka Infrastructure

### Setup
- Confluent Kafka Go client (`confluent-kafka-go`)
- Producer in `internal/kafka/producer/` — publish message to any topic
- Topic constants: `notifyx.email`, `notifyx.push`, `notifyx.sms`, `notifyx.inapp`, `notifyx.dlq`

### Consumer Base
- Generic consumer worker loop in `internal/kafka/consumers/`
- Handles: message polling, deserialization, dispatch to handler func
- Built-in retry: attempt handler up to 3x with exponential backoff (1s, 2s, 4s)
- After 3 failures: publish original message to `notifyx.dlq`

### DLQ Consumer
- Consumes `notifyx.dlq`
- Persists to `dlq_messages` table via repository
- Logs structured error with notification ID, channel, attempt count

### Channel Consumer Stubs
- Email consumer: reads `notifyx.email`, logs "stub delivery", updates status to delivered
- Push consumer: reads `notifyx.push`, same stub
- SMS consumer: reads `notifyx.sms`, same stub
- In-app consumer: reads `notifyx.inapp`, same stub

---

## Phase 4 — Notification Core

### Repository
- `NotificationRepository`: Create, GetByID, GetByTenantID (paginated), UpdateStatus
- `NotificationDeliveryRepository`: Create, UpdateStatus, GetByNotificationID

### Send Pipeline
- Validate request (required fields, channel enabled for tenant)
- Persist notification (status: pending)
- Persist delivery row (status: queued)
- Publish to Kafka topic for channel
- Return notification ID in response

### API
- `POST /api/v1/notifications/send` — single send
- `POST /api/v1/notifications/batch` — array of recipients, persist + publish one per recipient
- `GET /api/v1/notifications/history` — paginated, filterable by channel/status/date
- `GET /api/v1/notifications/:id` — single notification + delivery status

### Priority
- Include priority in Kafka message header
- Consumers process critical/high before normal/low (Kafka partition key = priority)

---

## Phase 5 — Rate Limiting & Deduplication

### Redis Setup
- Upstash Redis client in `internal/ratelimit/` and `internal/dedup/`
- Connection via `REDIS_URL` env var

### Rate Limiter
- Sliding window using Redis sorted sets
- Per tenant per channel: `rate:{tenantId}:{channel}` — configurable max/min
- Per tenant global: `rate:{tenantId}:global` — configurable global cap
- When limit hit: persist notification with status `queued_rate_limited`, schedule for retry after window resets
- Expose `Allow(tenantID, channel) bool` method

### Deduplication
- `dedup:{idempotencyKey}` → notification ID, TTL 24h
- On duplicate: return original notification ID with `200 OK` (not an error)
- Applied in send handler before any DB write

---

## Phase 6 — Channel Integrations

### Email (Resend)
- Resend Go SDK or HTTP client
- Build email payload from notification (to, subject, body)
- On success: update delivery status to `delivered`, set `delivered_at`
- On failure: return error to consumer retry loop

### Push (Firebase FCM)
- Firebase Admin Go SDK
- Build FCM message from notification (token, title, body)
- On success: update delivery status
- On failure: return error

### SMS (Fast2SMS)
- HTTP client to Fast2SMS REST API
- Build SMS payload (phone, message)
- On success: update delivery status
- On failure: return error

### Delivery Receipts (all channels)
- After successful delivery: `UPDATE notification_deliveries SET status='delivered', delivered_at=now(), attempts=N`
- After final failure (before DLQ): `UPDATE notification_deliveries SET status='failed', error_message=err`

---

## Phase 7 — WebSocket & In-app

### WebSocket Server
- Gorilla WebSocket upgrade in `internal/websocket/`
- `GET /ws/connect?tenantId=&userId=` — upgrade to WS
- Connection registry: map of `userID → *websocket.Conn`

### Presence Tracking
- On connect: `SET presence:{tenantId}:{userId} 1 EX 86400` in Redis
- On disconnect: `DEL presence:{tenantId}:{userId}`
- Expose `IsOnline(tenantID, userID string) bool`

### Offline Queue
- On in-app notification, check presence
- If online: send directly over WS connection
- If offline: `LPUSH offline:{userId} <notification_json>` with 7d TTL
- On reconnect: `LRANGE offline:{userId} 0 -1` → send all → `DEL offline:{userId}`

### In-app Consumer
- Replace stub with real WS delivery
- Call `IsOnline` → deliver or queue

---

## Phase 8 — Templates

### Repository
- `TemplateRepository`: Create, GetByID, GetByTenantID, Update, Delete

### API
- `POST /api/v1/templates`
- `GET /api/v1/templates` — list by tenant
- `PUT /api/v1/templates/:id`
- `DELETE /api/v1/templates/:id`

### Rendering
- Simple `{{variable}}` substitution in `internal/templates/`
- Accept `variables map[string]string` in send request
- If `template_id` provided: fetch template, render body (and subject for email), use rendered output in Kafka message

---

## Phase 9 — Scheduler & Cleanup

### Scheduler
- Cron runner in `internal/scheduler/` using `robfig/cron`
- Every 30s: `SELECT * FROM scheduled_notifications WHERE scheduled_at <= now() AND fired = false`
- For each: mark `fired = true`, publish to channel Kafka topic
- Handle concurrent cron ticks safely (DB update with WHERE fired = false to prevent double-fire)

### API
- `POST /api/v1/notifications/schedule` — same as send but with `scheduled_at`, persists to both `notifications` and `scheduled_notifications`

### Cleanup Job
- Daily cron: `DELETE FROM notifications WHERE expires_at < now()`
- Log count of deleted rows

---

## Phase 10 — Analytics

### Repository
- Queries against `notifications` and `notification_deliveries` tables per tenant
- Total sent/delivered/failed (COUNT grouped by status)
- Delivery rate per channel
- DLQ count per day (last 30 days) from `dlq_messages`
- Channel breakdown (COUNT grouped by channel)

### API
- `GET /api/v1/tenants/:id/analytics` — returns all above as single JSON response

---

## Phase 11 — Observability

### Logging (Zap)
- Add structured log lines to all handlers: request in, response out, errors
- Add to all Kafka consumers: message received, delivery success/failure, retry attempt, DLQ

### Prometheus Metrics
- Counter: `notifyx_notifications_sent_total{channel, tenant, priority}`
- Counter: `notifyx_notifications_delivered_total{channel, tenant}`
- Counter: `notifyx_notifications_failed_total{channel, tenant}`
- Counter: `notifyx_dlq_messages_total{channel}`
- Histogram: `notifyx_delivery_duration_seconds{channel}`
- Gauge: `notifyx_ws_connections_active`
- Expose `/metrics` endpoint

### OpenTelemetry
- Trace spans: HTTP handler → Kafka publish → Kafka consume → channel provider call
- Export to OTLP endpoint (local Jaeger in docker-compose)

### Grafana
- `docker-compose.yml` includes Grafana + Prometheus
- Pre-built dashboard JSON for all Notifyx metrics

---

## Phase 12 — Flutter Dashboard

### Setup
- Flutter Web project in `frontend/flutter/`
- Bloc + HTTP client + WebSocket client
- API base URL from environment config

### Screens
- **Overview**: metric cards (total sent, delivered, failed, delivery rate) + channel pie chart
- **Send Notification**: form with channel selector, recipient fields, template selector, priority, send button
- **Notification History**: paginated table, filter by channel/status/date, click row to see delivery detail
- **Templates**: list + create/edit/delete modal
- **Tenants**: list + create tenant, manage channel opt-ins, set rate limits per channel
- **Analytics**: line chart (delivery rate over time), bar chart (channel breakdown), DLQ trend chart

---

## Phase 13 — Deployment

### Docker
- Multi-stage `Dockerfile`: build stage (Go) + minimal runtime stage
- `docker-compose.yml`: Go service, PostgreSQL, Redis, Kafka (Confluent local), Grafana, Prometheus, Jaeger

### Render
- `render.yaml`: web service (Go binary) + PostgreSQL addon
- Health check endpoint: `GET /health`
- Environment variables documented in `.env.example`

### README
- Project overview + architecture diagram reference
- Local dev setup (docker-compose up)
- Render deployment steps
- API usage examples (curl)
