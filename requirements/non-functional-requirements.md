# Notifyx — Non-Functional Requirements

## Reliability
- Zero event loss: every notification must either be delivered or land in the DLQ
- Idempotent message processing across Kafka consumers
- Fault-tolerant retry with exponential backoff (3 attempts before DLQ)
- Offline message persistence for in-app channel

## Scalability
- Single deployable service (monolith with internal modularity)
- Kafka decouples ingestion from delivery — consumers can be scaled independently in future
- Redis sliding window rate limiting handles burst traffic gracefully

## Performance
- WebSocket connections for real-time in-app delivery (low latency)
- Redis caching for presence tracking and offline queues
- Async processing via Kafka — API responds immediately after enqueueing

## Observability
- Structured JSON logging via Uber Zap
- Prometheus metrics: notification counts, delivery rates, retry counts, DLQ counts, latency
- OpenTelemetry distributed tracing across API → Kafka → consumer → channel
- Grafana dashboard surfaces all metrics

## Security
- API key authentication per tenant (hashed in DB)
- Versioned API (`/api/v1/`)
- No credentials stored in plaintext

## Frontend Scope
- Flutter dashboard is desktop-only for now (no responsive/mobile layout)
- Mobile support out of scope in current version

## Maintainability
- Clean internal module separation (channels, kafka, ratelimit, etc.)
- Single deployable simplifies ops
- Docker Compose for reproducible local dev
- DB migrations versioned under `/migrations`

## Deployment
- Fully containerized (Dockerfile)
- Environment-based config (no hardcoded values)
- Free-tier only: Render, Confluent Kafka, Upstash Redis, Resend, Firebase FCM, Fast2SMS
