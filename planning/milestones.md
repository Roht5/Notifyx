# Notifyx — Milestones

## Phase 1 — Foundation
Project scaffold, configuration, database, domain models, logger.
- [x] Initialize Go module and folder structure
- [x] Environment-based config (config.go)
- [x] Uber Zap logger setup
- [x] PostgreSQL connection + migration runner
- [x] All DB migrations (schema)
- [x] Domain models (Notification, Tenant, Template, DLQMessage, etc.)

## Phase 2 — Tenant & Auth
Tenant management and API key authentication.
- [x] Tenant CRUD (POST, GET, PUT, DELETE /api/v1/tenants)
- [x] API key generation + hashing (SHA-256, not bcrypt — see apikey.go comment)
- [x] API key auth middleware
- [x] Tenant channel opt-in management
- [x] Tenant rate limit configuration

## Phase 3 — Kafka Infrastructure
Producer and consumer scaffolding before any channel is wired up.
- [x] Kafka client setup (segmentio/kafka-go, not confluent-kafka-go — see decisions.md)
- [x] Producer (publish to per-channel topics)
- [x] Consumer base (reusable worker loop with retry + backoff)
- [x] DLQ consumer (reads notifyx.dlq, persists to dlq_messages)
- [x] Per-channel consumer stubs (email, push, sms, inapp)
- [x] Conditional startup wiring in main.go (KAFKA_BOOTSTRAP_SERVERS) + graceful shutdown

## Phase 4 — Notification Core
Send, batch, history, and delivery tracking — without channel providers yet (stub delivery).
- [x] POST /api/v1/notifications/send
- [x] POST /api/v1/notifications/batch
- [x] GET /api/v1/notifications/history
- [x] GET /api/v1/notifications/:id
- [x] Notification + delivery persistence in PostgreSQL
- [x] Priority ordering in Kafka messages (carried as the partition key, set in Phase 3)

## Phase 5 — Rate Limiting & Deduplication
Redis-based safeguards applied to the send pipeline.
- [x] Upstash Redis client setup
- [x] Sliding window rate limiter (per tenant per channel + global cap)
- [x] Rate limit check wired into send/batch handlers (degrades to "always allow" if REDIS_URL unset)
- [x] Queuing logic when limit is hit (store as queued_rate_limited + background replayer)
- [x] Idempotency key deduplication (Redis, 24h TTL)

## Phase 6 — Channel Integrations
Wire up real providers into the Kafka consumers.
- [x] Email — Resend integration
- [x] Push — Firebase FCM integration
- [x] SMS — Fast2SMS integration
- [x] Delivery receipt updates (status, delivered_at, attempts)

## Phase 7 — WebSocket & In-app
Real-time in-app delivery with presence and offline support.
- [x] Gorilla WebSocket server
- [x] WS /ws/connect?tenantId=&userId= endpoint
- [x] Presence tracking in Redis
- [x] Online delivery (push directly over WS)
- [x] Offline queue (Redis list, flush on reconnect)
- [x] In-app Kafka consumer wired to WS delivery

## Phase 8 — Templates
Template CRUD and rendering at send time.
- [x] POST/GET/PUT/DELETE /api/v1/templates
- [x] Template storage in PostgreSQL
- [x] Template rendering engine ({{variable}} substitution)
- [x] Wire template rendering into all channel consumers (rendered once at send time, before publish — see decisions.md)

## Phase 9 — Scheduler & Cleanup
Scheduled notifications and 90-day history cleanup.
- [x] POST /api/v1/notifications/schedule
- [x] Cron runner (every 30s) — poll + publish due notifications
- [x] Mark scheduled_notifications.fired = true after publish
- [x] Daily cleanup cron — delete notifications where expires_at < now()

## Phase 10 — Analytics
Per-tenant analytics API consumed by the Flutter dashboard.
- [x] GET /api/v1/tenants/:id/analytics
- [x] Total sent, delivered, failed counts
- [x] Delivery rate per channel
- [x] DLQ trend over time
- [x] Channel breakdown

## Phase 11 — Observability
Structured logging, metrics, and tracing wired across all layers.
- [ ] Zap structured logging in all handlers + consumers
- [ ] Prometheus metrics (notification counts, delivery rates, retry counts, DLQ counts, latency)
- [ ] OpenTelemetry tracing (API → Kafka → consumer → channel)
- [ ] Grafana dashboard config (docker-compose local)

## Phase 12 — Flutter Dashboard
All 6 screens using Bloc state management.
- [ ] Project setup (Flutter Web, Bloc, API client)
- [ ] Overview screen (metrics cards + charts)
- [ ] Send Notification screen (test send form)
- [ ] Notification History screen (table + filters)
- [ ] Templates screen (CRUD)
- [ ] Tenants screen (manage tenants, rate limits, channel opt-ins)
- [ ] Analytics screen (charts — delivery rate, channel breakdown, DLQ trends)

## Phase 13 — Deployment
Containerization and Render deployment.
- [ ] Dockerfile (multi-stage Go build)
- [ ] docker-compose.yml (Go + PostgreSQL + Redis + Kafka + Grafana)
- [ ] .env.example
- [ ] Render deployment config (render.yaml)
- [ ] Move `postgres.RunMigrations` out of `main()` into Render's Pre-Deploy Command (see decisions.md, "migrations-on-startup")
- [ ] README with setup instructions
