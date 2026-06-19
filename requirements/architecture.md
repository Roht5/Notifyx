# Notifyx — Architecture

## High-Level Flow

```
Client → POST /api/v1/notifications/send
       → Rate limit check (Redis sliding window, per tenant per channel + global)
       → Dedup check (Redis idempotency key, 24h TTL)
       → Persist to PostgreSQL (notifications table)
       → Publish to Kafka topic (per channel)
       → Kafka consumer picks up
           → Render template (if template_id provided)
           → Call channel provider (Resend / FCM / Fast2SMS / WebSocket)
           → On success → update delivery status in PostgreSQL
           → On failure → retry up to 3x with exponential backoff
               → After 3 failures → publish to notifyx.dlq topic
               → DLQ consumer → persist to dlq_messages table
```

## Scheduled Notification Flow

```
Client → POST /api/v1/notifications/schedule (with scheduled_at)
       → Persist to notifications table (status: pending) + scheduled_notifications table
       → Internal cron (every 30s) polls scheduled_notifications where scheduled_at <= now() AND fired = false
       → Marks row as fired = true
       → Publishes directly to per-channel Kafka topic (notifyx.email / push / sms / inapp)
       → Same delivery pipeline as immediate send from here
```

Note: no separate `notifyx.scheduled` Kafka topic — scheduler publishes directly to channel topics.

## Batch Send Flow

```
Client → POST /api/v1/notifications/batch (array of recipients, single message)
       → Rate limit check (counted as N notifications against tenant limit)
       → Dedup check per recipient (idempotency_key + recipientId)
       → Persist one notification row per recipient
       → Publish one Kafka message per recipient to channel topic
       → Each message processed independently through delivery pipeline
```

## WebSocket / In-app Flow

```
Client → WS /ws/connect?tenantId=&userId=
       → Register presence in Redis (presence:{tenantId}:{userId})
       → If offline queue has pending messages → flush immediately
       → On disconnect → remove presence key
       → On new in-app notification → check presence
           → Online: push via WebSocket
           → Offline: push to Redis offline queue (offline:{userId})
       → On reconnect → flush offline queue
```

## Kafka Topics

| Topic | Purpose |
|---|---|
| `notifyx.email` | Email delivery jobs |
| `notifyx.push` | Push delivery jobs |
| `notifyx.sms` | SMS delivery jobs |
| `notifyx.inapp` | In-app delivery jobs |
| `notifyx.dlq` | Failed messages after 3 retry attempts |

## Redis Key Design

| Key | Type | TTL | Purpose |
|---|---|---|---|
| `rate:{tenantId}:{channel}` | Sorted Set | 60s | Sliding window per channel |
| `rate:{tenantId}:global` | Sorted Set | 60s | Sliding window global cap |
| `dedup:{idempotencyKey}` | String | 24h | Deduplication |
| `presence:{tenantId}:{userId}` | String | Session | Online/offline flag |
| `offline:{userId}` | List | 7d | Pending in-app notifications |

## PostgreSQL Schema

```sql
tenants
  id UUID PK, name TEXT, global_rate_cap INT, created_at TIMESTAMP

tenant_channels
  id UUID PK, tenant_id UUID FK, channel TEXT, enabled BOOL

tenant_rate_limits
  id UUID PK, tenant_id UUID FK, channel TEXT, max_per_min INT
  -- global_rate_cap lives on tenants, not here — it's one value per tenant, not one
  -- per channel row (see decisions.md, "tenant_rate_limits 3NF fix")

api_keys
  id UUID PK, tenant_id UUID FK, key_hash TEXT, created_at TIMESTAMP

notification_templates
  id UUID PK, tenant_id UUID FK, name TEXT, channel TEXT, subject TEXT, body TEXT, created_at TIMESTAMP

notifications
  id UUID PK, tenant_id UUID FK, channel TEXT, priority TEXT, status TEXT,
  recipient_id TEXT, recipient_email TEXT, recipient_phone TEXT, recipient_token TEXT,
  template_id UUID FK nullable, subject TEXT, body TEXT, metadata JSONB,
  idempotency_key TEXT, scheduled_at TIMESTAMP nullable,
  created_at TIMESTAMP, expires_at TIMESTAMP (90 days)

notification_deliveries
  id UUID PK, notification_id UUID FK, channel TEXT, status TEXT,
  attempts INT, error_message TEXT, delivered_at TIMESTAMP nullable, created_at TIMESTAMP

scheduled_notifications
  id UUID PK, notification_id UUID FK, scheduled_at TIMESTAMP, fired BOOL, created_at TIMESTAMP

dlq_messages
  id UUID PK, notification_id UUID FK, channel TEXT, error TEXT,
  attempts INT, last_tried_at TIMESTAMP, created_at TIMESTAMP
```

## API Routes

```
POST   /api/v1/notifications/send
POST   /api/v1/notifications/schedule
POST   /api/v1/notifications/batch
GET    /api/v1/notifications/history
GET    /api/v1/notifications/:id

POST   /api/v1/templates
GET    /api/v1/templates
PUT    /api/v1/templates/:id
DELETE /api/v1/templates/:id

POST   /api/v1/tenants
GET    /api/v1/tenants
PUT    /api/v1/tenants/:id
DELETE /api/v1/tenants/:id
GET    /api/v1/tenants/:id/analytics

WS     /ws/connect?tenantId=&userId=
```

## Project Structure

```
notifyx/
├── backend/                 # Go service (cd here before running Go commands)
│   ├── cmd/server/          # entrypoint
│   ├── config/              # env-based config
│   ├── internal/
│   │   ├── api/
│   │   │   ├── handlers/    # Echo route handlers
│   │   │   ├── middleware/  # auth, logging, rate limit
│   │   │   └── routes/      # server setup + route registration
│   │   ├── websocket/       # Gorilla WS server + presence tracking
│   │   ├── kafka/
│   │   │   ├── producer/    # publish to Kafka topics
│   │   │   └── consumers/   # per-channel consumers + DLQ consumer
│   │   ├── scheduler/       # cron runner for scheduled notifications
│   │   ├── channels/
│   │   │   ├── email/       # Resend integration
│   │   │   ├── push/        # Firebase FCM integration
│   │   │   ├── sms/         # Fast2SMS integration
│   │   │   └── inapp/       # WebSocket delivery
│   │   ├── ratelimit/       # Redis sliding window
│   │   ├── dedup/           # Redis idempotency
│   │   ├── dlq/             # DLQ consumer + storage
│   │   ├── templates/       # template rendering
│   │   ├── repository/
│   │   │   └── postgres/    # all DB queries
│   │   └── domain/          # shared types (Notification, Tenant, etc.)
│   ├── pkg/
│   │   ├── logger/          # Zap setup
│   │   └── telemetry/       # OTel + Prometheus
│   ├── migrations/          # versioned SQL migrations
│   ├── go.mod
│   └── Dockerfile
├── frontend/
│   └── flutter/             # Flutter Web frontend (Phase 12)
├── docs/                    # project context docs
├── planning/                # milestones, task breakdown, decisions
├── requirements/            # this folder
├── skills/                  # reusable skill guides
└── docker-compose.yml       # orchestrates all services (root-level)
```

## Flutter Dashboard Screens

| Screen | Description |
|---|---|
| Overview | Total sent, delivery rate, failures, channel breakdown charts |
| Send Notification | Test send form — channel, template, recipient, priority |
| Notification History | Table with status, channel, timestamp, retry count |
| Templates | Create/edit templates with variable placeholders |
| Tenants | Manage tenants, toggle channel opt-ins, set rate limits |
| Analytics | Delivery rate over time, per-channel charts, DLQ trends |
