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
- [ ] Tenant CRUD (POST, GET, PUT, DELETE /api/v1/tenants)
- [ ] API key generation + hashing (bcrypt)
- [ ] API key auth middleware
- [ ] Tenant channel opt-in management
- [ ] Tenant rate limit configuration

## Phase 3 — Kafka Infrastructure
Producer and consumer scaffolding before any channel is wired up.
- [ ] Confluent Kafka client setup
- [ ] Producer (publish to per-channel topics)
- [ ] Consumer base (reusable worker loop with retry + backoff)
- [ ] DLQ consumer (reads notifyx.dlq, persists to dlq_messages)
- [ ] Per-channel consumer stubs (email, push, sms, inapp)

## Phase 4 — Notification Core
Send, batch, history, and delivery tracking — without channel providers yet (stub delivery).
- [ ] POST /api/v1/notifications/send
- [ ] POST /api/v1/notifications/batch
- [ ] GET /api/v1/notifications/history
- [ ] GET /api/v1/notifications/:id
- [ ] Notification + delivery persistence in PostgreSQL
- [ ] Priority ordering in Kafka messages

## Phase 5 — Rate Limiting & Deduplication
Redis-based safeguards applied to the send pipeline.
- [ ] Upstash Redis client setup
- [ ] Sliding window rate limiter (per tenant per channel + global cap)
- [ ] Rate limit middleware wired into send/batch routes
- [ ] Queuing logic when limit is hit (store + replay)
- [ ] Idempotency key deduplication (Redis, 24h TTL)

## Phase 6 — Channel Integrations
Wire up real providers into the Kafka consumers.
- [ ] Email — Resend integration
- [ ] Push — Firebase FCM integration
- [ ] SMS — Fast2SMS integration
- [ ] Delivery receipt updates (status, delivered_at, attempts)

## Phase 7 — WebSocket & In-app
Real-time in-app delivery with presence and offline support.
- [ ] Gorilla WebSocket server
- [ ] WS /ws/connect?tenantId=&userId= endpoint
- [ ] Presence tracking in Redis
- [ ] Online delivery (push directly over WS)
- [ ] Offline queue (Redis list, flush on reconnect)
- [ ] In-app Kafka consumer wired to WS delivery

## Phase 8 — Templates
Template CRUD and rendering at send time.
- [ ] POST/GET/PUT/DELETE /api/v1/templates
- [ ] Template storage in PostgreSQL
- [ ] Template rendering engine ({{variable}} substitution)
- [ ] Wire template rendering into all channel consumers

## Phase 9 — Scheduler & Cleanup
Scheduled notifications and 90-day history cleanup.
- [ ] POST /api/v1/notifications/schedule
- [ ] Cron runner (every 30s) — poll + publish due notifications
- [ ] Mark scheduled_notifications.fired = true after publish
- [ ] Daily cleanup cron — delete notifications where expires_at < now()

## Phase 10 — Analytics
Per-tenant analytics API consumed by the Flutter dashboard.
- [ ] GET /api/v1/tenants/:id/analytics
- [ ] Total sent, delivered, failed counts
- [ ] Delivery rate per channel
- [ ] DLQ trend over time
- [ ] Channel breakdown

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
- [ ] README with setup instructions
