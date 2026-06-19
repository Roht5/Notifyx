# Notifyx — Decisions Log

All decisions made during planning. Do not revisit these unless the developer explicitly asks.

---

## Architecture

**Single deployable service, not microservices**
Chosen for simplicity of deployment on Render free tier and to keep the project manageable solo. Internal modularity (clean package separation) ensures it can be split later if needed.

**Echo over Gin or Fiber**
Echo has cleaner middleware support, good documentation, and a well-established ecosystem. Gin is slightly faster but the difference is negligible for this project. Fiber has a non-standard API that diverges from net/http conventions.

**Gorilla WebSocket over gobwas/ws**
Gorilla is the most mature and widely used Go WebSocket library. gobwas/ws is faster but lower-level and less beginner-friendly — important since Go is new to the developer.

**One Kafka topic per channel (not unified)**
Cleaner separation of concerns. Independent consumer groups per channel. Easier to debug and scale independently in future. A unified topic with routing logic inside would couple all channel consumers together.

**Single DLQ topic (not per-channel)**
Simplifies DLQ monitoring and storage. Failed messages carry their channel as metadata so they can be reprocessed per channel if needed.

**Scheduler polls DB directly, publishes to channel topics**
Simpler than a separate `notifyx.scheduled` Kafka topic. The scheduler is a thin cron layer — adding a Kafka intermediary adds complexity with no benefit at this scale.

**segmentio/kafka-go over confluent-kafka-go (deviation from original plan)**
confluent-kafka-go requires librdkafka (a CGO/C library) which wasn't available in the dev environment and complicates Docker builds. segmentio/kafka-go is pure Go, speaks the same wire protocol, and supports SASL_SSL/PLAIN against Confluent Cloud — no functional loss for this project's needs.

---

## Channels

**Email: Resend (not SendGrid or Mailgun)**
Resend has the cleanest API, good Go SDK, and 100 free emails/day which is sufficient for a portfolio project.

**Push: Firebase FCM**
Free, widely used, straightforward Admin SDK for Go. No viable free alternative.

**SMS: Fast2SMS (not Twilio or Vonage)**
Developer is based in India. Fast2SMS has a free tier for Indian numbers and a simple REST API. Twilio's free trial is credit-based (not ongoing free). Vonage free credits expire.

**In-app: WebSocket (not SSE)**
WebSocket was explicitly chosen over Server-Sent Events for resume impact — bidirectional connection is a stronger signal in interviews.

---

## Data & Caching

**Upstash Redis (not self-hosted)**
Free tier is generous for this use case. Upstash is serverless Redis — no instance management. Works well with Render deployment.

**Confluent Kafka (not Redpanda or self-hosted)**
Confluent has a proper free tier (5GB/month). Redpanda is Kafka-compatible but less recognized by name on a resume. Self-hosted Kafka on Render free tier is not feasible.

**PostgreSQL for notification history (not Redis or MongoDB)**
Structured, queryable, supports the analytics queries needed. 90-day retention is manageable with indexed timestamps.

**90-day notification retention**
Balances storage cost (free tier has limits) with enough history for meaningful analytics. Implemented via `expires_at` column + daily cleanup cron.

---

## Rate Limiting

**Sliding window (not fixed window or token bucket)**
Sliding window is the most accurate and fair approach. Fixed window has burst edge cases. Token bucket is overkill for per-minute limits.

**Queue when rate limited (not drop or error)**
Dropping silently loses data. Returning an error pushes retry responsibility to the caller. Queuing for later delivery is the most resilient approach and matches real-world notification systems.

**Per tenant per channel + global cap**
Both are needed. Per-channel prevents one noisy channel from consuming all quota. Global cap prevents a single tenant from overwhelming the system across all channels.

---

## Multi-tenancy

**Tenant opt-in per channel (not opt-out)**
Explicit opt-in is safer — tenants only send via channels they have deliberately enabled. Reduces accidental sends.

**API key auth (not JWT)**
API keys are simpler for service-to-service use (the primary use case). JWTs make sense for user-facing auth — not applicable here. Dashboard is open (no auth) for now.

**No tenant-owned channel credentials**
Each tenant does NOT bring their own Resend/FCM keys. All sends go through Notifyx's shared provider accounts. Simplifies implementation significantly. Can be added later.

---

## Frontend

**Flutter Web (not React)**
Developer has strong Flutter experience from Zenoti. Using Flutter Web is faster to build and consistent with his existing skill set.

**Bloc (not Riverpod or Provider)**
Developer's stated preference. Bloc is also the most structured and interview-recognizable Flutter state management pattern.

**Desktop-only (no responsive layout)**
Dashboard is a developer/admin tool. Mobile layout is out of scope for this version.

**Open dashboard (no login)**
Auth adds complexity without adding portfolio value. Can be added later. The backend API key auth is the relevant security feature to showcase.

---

## Observability

**Uber Zap (not zerolog or logrus)**
Zap is the most widely used structured logger in Go. High performance, good developer experience.

**Prometheus + Grafana (not Datadog or New Relic)**
Free, self-hostable, industry standard. Developer has New Relic experience from Zenoti — using a different tool here adds breadth.

**OpenTelemetry (not custom tracing)**
OTel is the vendor-neutral standard. Shows awareness of modern observability practices.

---

## Deployment

**Render (not Railway or Fly.io)**
Render has a reliable free tier for both web services and PostgreSQL. Railway's free tier is more restrictive. Fly.io requires a credit card.

**Docker Compose for local dev**
Reproducible local environment. Single command to spin up all dependencies. Standard practice for distributed systems projects.
