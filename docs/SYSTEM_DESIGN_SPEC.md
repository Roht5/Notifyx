# Notifyx — Complete System Architecture & Developer Specification

This document is the absolute source of truth for the **Notifyx** notification platform. It provides exhaustive technical specifications, database designs, messaging flows, memory layouts, coding standards, and operational guidelines. 

It is designed to be fully parseable and actionable for both **human software engineers** and **AI coding agents** (e.g., Claude Code) to build, test, and maintain the service across all development phases.

---

## 🗺️ 1. Global Architecture Specification

Notifyx is structured as a single-binary service containing two decoupled execution layers: a synchronous HTTPS ingestion gateway and a pool of asynchronous event-driven worker consumers.

```
                           +------------------------+
                           |     Client Request     |
                           +-----------+------------+
                                       |
                                       | HTTPS (X-API-Key)
                                       ▼
                     +-----------------------------------+
                     |   Ingestion Gateway (Echo API)    |
                     +─────────────────┬─────────────────+
                                       │
            ┌──────────────────────────┼──────────────────────────┐
            ▼                          ▼                          ▼
     [ API Key Auth ]          [ Rate Limiter ]            [ Deduplicator ]
  Verify X-API-Key Hash       Check Sliding Window       Verify Idempotency
  against Redis / Postgres    in Redis (Global + Chan)   Hash Key (SetNX Lua)
            │                          │                          │
            └──────────────────────────┼──────────────────────────┘
                                       ▼
                       +───────────────────────────────+
                       |  PostgreSQL DB Transaction    |
                       |  - Create Notification        |
                       |  - Create Delivery Attempt    |
                       +───────────────┬───────────────+
                                       │
                                       ▼ (Commit & Publish)
                       +───────────────────────────────+
                       |        Kafka Producer         |
                       +───────────────┬───────────────+
                                       │
         ┌──────────────────────┬──────┴───────────────┬──────────────────────┐
         ▼                      ▼                      ▼                      ▼
   notifyx.email          notifyx.sms            notifyx.push          notifyx.inapp
         │                      │                      │                      │
         ▼                      ▼                      ▼                      ▼
  [Email Consumer]       [SMS Consumer]         [Push Consumer]        [In-App Consumer]
  (Resend API 3x Retries)(Fast2SMS 3x Retries)  (FCM SDK 3x Retries)   (Gorilla WS Server)
         │                      │                      │                      │
         └──────────────────────┼──────────────────────┘                      │
                                ▼ (All 3 retries failed)                      │
                      +───────────────────+                                   ▼
                      |    notifyx.dlq    |                                [ Redis ]
                      +─────────┬─────────+                          (7d Offline Queue)
                                │
                                ▼
                        [ DLQ Consumer ]
                                │
                                ▼
                       [ Postgres Persist ]
                        (dlq_messages DDL)
```

### 1.1 Notification Lifecycle States
* **`pending`**: Notification created, database record committed, event successfully published to Kafka.
* **`queued`**: In-flight in the consumer queue, awaiting delivery execution by the worker.
* **`queued_rate_limited`**: Tenant rate limits exceeded. Held in database, bypassed Kafka queue, awaiting replayer dispatch.
* **`delivered`**: Successful transmission confirmed by the third-party channel provider.
* **`failed`**: Delivery failed after 3 retry attempts, or rejected due to configuration error.

---

## 🗄️ 2. Database Specification & DDL Schema

PostgreSQL (managed connection pool via `pgx/v5`) is the database of record. Timestamps MUST be stored as `TIMESTAMPTZ` (UTC).

### 2.1 Schema Definition (DDL)

```sql
-- 1. Tenants Table
-- Core tenant record. global_rate_cap controls maximum aggregate sends/min.
CREATE TABLE tenants (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT        NOT NULL,
    global_rate_cap  INT         NOT NULL DEFAULT 300,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Tenant Channels Table
-- Records channel enablement. A tenant cannot send via disabled channels.
CREATE TABLE tenant_channels (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel    TEXT        NOT NULL, -- 'email', 'push', 'sms', 'inapp'
    enabled    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, channel)
);
CREATE INDEX idx_tenant_channels_tenant_id ON tenant_channels(tenant_id);

-- 3. Tenant Rate Limits Table
-- Defines channel-specific throughput thresholds.
CREATE TABLE tenant_rate_limits (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel      TEXT        NOT NULL, -- 'email', 'push', 'sms', 'inapp'
    max_per_min  INT         NOT NULL DEFAULT 60,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, channel)
);
CREATE INDEX idx_tenant_rate_limits_tenant_id ON tenant_rate_limits(tenant_id);

-- 4. API Keys Table
-- API keys are cryptographically random strings. Only the SHA-256 hex hash is stored.
CREATE TABLE api_keys (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key_hash   TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);

-- 5. Notification Templates Table
-- Supports custom dynamic variable rendering. Name must be unique per tenant.
CREATE TABLE notification_templates (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    channel    TEXT        NOT NULL,
    subject    TEXT,                 -- Only populated for email
    body       TEXT        NOT NULL, -- Contains {{variable}} syntax
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);
CREATE INDEX idx_notification_templates_tenant_id ON notification_templates(tenant_id);

-- 6. Notifications Table
-- Central table. metadata uses JSONB. expires_at defaults to 90 days.
CREATE TABLE notifications (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel          TEXT        NOT NULL,
    priority         TEXT        NOT NULL DEFAULT 'normal', -- 'critical', 'high', 'normal', 'low'
    status           TEXT        NOT NULL DEFAULT 'pending',
    recipient_id     TEXT,
    recipient_email  TEXT,
    recipient_phone  TEXT,
    recipient_token  TEXT,
    template_id      UUID        REFERENCES notification_templates(id) ON DELETE SET NULL,
    subject          TEXT,
    body             TEXT        NOT NULL,
    metadata         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key  TEXT,
    scheduled_at     TIMESTAMPTZ, -- Null for immediate sends
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '90 days'),
    CONSTRAINT notifications_tenant_idempotency_key_key UNIQUE (tenant_id, idempotency_key)
);
CREATE INDEX idx_notifications_tenant_id ON notifications(tenant_id);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_created_at ON notifications(created_at);

-- Composite index optimized for paginated search history: GET /history
CREATE INDEX idx_notifications_tenant_created_at ON notifications(tenant_id, created_at DESC);

-- 7. Notification Deliveries Table
-- Audit log of individual attempts. Bumps attempts column on retries.
CREATE TABLE notification_deliveries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel         TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'queued',
    attempts        INT         NOT NULL DEFAULT 0,
    error_message   TEXT,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notification_deliveries_notification_id ON notification_deliveries(notification_id);

-- 8. Scheduled Notifications Table
-- Swept by robfig/cron runner.
CREATE TABLE scheduled_notifications (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    scheduled_at    TIMESTAMPTZ NOT NULL,
    fired           BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_scheduled_notifications_fired_at ON scheduled_notifications(scheduled_at) WHERE fired = FALSE;

-- 9. Dead Letter Queue Table (DLQ)
-- Holds terminal errors after all retries exhaust.
CREATE TABLE dlq_messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel         TEXT        NOT NULL,
    error           TEXT        NOT NULL,
    attempts        INT         NOT NULL DEFAULT 0,
    last_tried_at   TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_dlq_messages_created_at ON dlq_messages(created_at);
```

---

## 💾 3. Redis Key Space & Data Structures

Redis (managed via `redis/v9`) stores rate limit logs, connection states, and deduplication tokens.

### 3.1 Redis Cluster Key Routing Guard
In Redis Cluster deployments, keys are routed to nodes using hash calculations. For multi-key queries (like Lua rate limiter scripts) to succeed, keys MUST share the same **hash tag** `{...}`. Wrap `tenant_id` in curly braces to route all of a tenant's keys to the exact same hash slot:

| Key Structure | Type | TTL | Purpose |
|---|---|---|---|
| `rate:{<tenant_id>}:<channel>` | Sorted Set (ZSET) | `60s` | Sliding-window log per channel. Members are `unix_nanos-UUID`. |
| `rate:{<tenant_id>}:global` | Sorted Set (ZSET) | `60s` | Sliding-window log global limit. Members are `unix_nanos-UUID`. |
| `dedup:{<SHA256_idempotency_key>}` | String | `24h` | Deduplication lock. Value is `notification_id`. |
| `presence:{<tenant_id>}:{<user_id>}` | String | `24h` | WebSocket online flag. |
| `offline:{<user_id>}` | List | `7d` | Backlog of in-app notifications to push on reconnect. |

---

## 📨 4. Kafka Event Broker Specification

Notifyx uses Kafka for decoupled event queueing. 
* **Write Durability:** The producer MUST connect using `RequiredAcks: kafka.RequireAll` (equivalent to `acks=all`).
* **Format:** Payloads are serialized using JSON.

### 4.1 Message Payload Struct
```go
// Message is the shared format for all notifyx.* topic events.
type Message struct {
	NotificationID uuid.UUID       `json:"notification_id"`
	DeliveryID     uuid.UUID       `json:"delivery_id"`
	TenantID       uuid.UUID       `json:"tenant_id"`
	Channel        domain.Channel  `json:"channel"`
	Priority       domain.Priority `json:"priority"`
	RecipientID    string          `json:"recipient_id,omitempty"`
	RecipientEmail string          `json:"recipient_email,omitempty"`
	RecipientPhone string          `json:"recipient_phone,omitempty"`
	RecipientToken string          `json:"recipient_token,omitempty"`
	Subject        string          `json:"subject,omitempty"`
	Body           string          `json:"body"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	Error          string          `json:"error,omitempty"` // populated on DLQ transfers
}
```

### 4.2 Partition Routing Strategy
While the message partition key is set to the event `Priority` (to isolate critical/high events into distinct partitions), **Kafka does not prioritize partitions on the consumer side**. 
* **True Priority Routing:** If strict high-priority processing guarantees are required, separate topics MUST be declared (e.g. `notifyx.email.critical` vs `notifyx.email.normal`). The consumer pool must query the critical queue first, falling back to the normal queue only when empty.

---

## 🚦 5. Rate Limiting & Deduplication Engines

### 5.1 Sliding Window Rate Limiting (Lua)
The rate limiter evaluates limits atomically in a single roundtrip to prevent race conditions (Double-Allow bugs) using the following Lua script:

```lua
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local channelMax = tonumber(ARGV[3])
local globalMax = tonumber(ARGV[4])
local member = ARGV[5]

-- 1. Trim logs older than the current sliding window threshold
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - window)
redis.call('ZREMRANGEBYSCORE', KEYS[2], 0, now - window)

-- 2. Count active logs inside the window
local channelCount = redis.call('ZCARD', KEYS[1])
local globalCount = redis.call('ZCARD', KEYS[2])

-- 3. Block if limits are reached
if channelCount >= channelMax or globalCount >= globalMax then
    return 0
end

-- 4. Record new request logs and set sliding expiry
redis.call('ZADD', KEYS[1], now, member)
redis.call('ZADD', KEYS[2], now, member)
local windowSec = math.floor(window / 1000000000) + 1
redis.call('EXPIRE', KEYS[1], windowSec)
redis.call('EXPIRE', KEYS[2], windowSec)

return 1
```

### 5.2 Atomic Deduplication (Lua)
Deduplication prevents duplicate processing of retried client requests by atomically checking for an existing idempotency lock:

```lua
-- KEYS[1] = dedup key, ARGV[1] = notification ID, ARGV[2] = TTL seconds
-- Returns {1, nil} on successful claim, or {0, existing_id} if already locked
if redis.call('SETNX', KEYS[1], ARGV[1]) == 1 then
    redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
    return {1, ''}
else
    return {0, redis.call('GET', KEYS[1])}
end
```

To protect Redis memory, the client-provided key MUST be converted to a SHA-256 hash before string concatenation:
```go
hash := sha256.Sum256([]byte(idempotencyKey))
redisKey := "dedup:" + hex.EncodeToString(hash[:])
```

---

## 🔌 6. Delivery Channels & Provider Integrations

### 6.1 Email Integration (Resend)
* **API Target:** `POST https://api.resend.com/emails`
* **Headers:** `Authorization: Bearer <resend_api_key>`
* **Payload:**
  ```json
  {
    "from": "Notifyx <onboarding@resend.dev>",
    "to": ["<recipient_email>"],
    "subject": "<subject>",
    "html": "<rendered_body>"
  }
  ```

### 6.2 Push Integration (Firebase FCM)
* **SDK:** `firebase.google.com/go/v4`
* **Configuration:** Initialized via `FIREBASE_CREDENTIALS_JSON` environment variable path pointing to the service account credentials JSON.
* **Message Payload:**
  ```go
  &messaging.Message{
      Token: msg.RecipientToken,
      Notification: &messaging.Notification{
          Title: msg.Subject,
          Body:  msg.Body,
      },
  }
  ```

### 6.3 SMS Integration (Fast2SMS)
* **API Target:** `POST https://www.fast2sms.com/dev/bulkV2`
* **Headers:** `authorization: <fast2sms_api_key>`
* **Payload:**
  ```json
  {
    "route": "q",
    "message_type": "txt",
    "language": "english",
    "message": "<body_text>",
    "numbers": "<recipient_phone>"
  }
  ```

### 6.4 In-App Integration (WebSocket)
* **Upgrade:** Upgrade HTTP requests on `GET /ws/connect?tenantId=&userId=` to WebSocket protocol using Gorilla WebSockets.
* **Presence:** On connect, set `presence:{tenantID}:{userID}` in Redis. On disconnect, delete the key.
* **Offline Handling:** If a notification is sent to a user who is offline, queue the message in the Redis list `offline:{userID}` (capped at a 7-day TTL). Flush and transmit all queued messages immediately upon reconnection.

---

## 📈 7. Observability, Logging, & Telemetry

### 7.1 Prometheus Metrics Spec
The service exposes a `/metrics` route containing these system metrics:

| Metric Name | Type | Labels | Description |
|---|---|---|---|
| `notifyx_notifications_sent_total` | Counter | `channel, tenant, priority` | Total notifications received. |
| `notifyx_notifications_delivered_total` | Counter | `channel, tenant` | Successful provider sends. |
| `notifyx_notifications_failed_total` | Counter | `channel, tenant` | Terminal failures sent to DLQ. |
| `notifyx_dlq_messages_total` | Counter | `channel` | Total messages moved to DLQ. |
| `notifyx_delivery_duration_seconds` | Histogram | `channel` | Network request delivery latency. |
| `notifyx_ws_connections_active` | Gauge | None | Active WebSocket client connections. |

### 7.2 OpenTelemetry Spans
OTel trace spans MUST propagate context across:
1. **HTTP Ingestion:** Handler entry -> API Key Auth -> DB Transaction -> Kafka Produce.
2. **Kafka Delivery Loop:** Consumer Fetch -> Provider Network Send -> DB Update Handoff.

---

## 💻 8. Developer Code Standards (Mandatory)

### 8.1 Database Interface Injection
To support PostgreSQL transactions, repositories MUST receive the `DB` interface instead of a direct `*pgxpool.Pool`:
```go
// Define in db.go
type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}
```
Instantiate your repository structs with this interface to enable transactions when needed:
```go
tx, err := pool.Begin(ctx)
txRepo := postgres.NewNotificationRepository(tx)
```

### 8.2 Avoid In-flight Duplication Panics
Do not use unchecked type assertions on Echo's context. Always handle type verification safely:
```go
func TenantFromContext(c echo.Context) (*domain.Tenant, bool) {
	val := c.Get("tenant")
	tenant, ok := val.(*domain.Tenant)
	return tenant, ok
}
```

### 8.3 Kafka Consumer Safe Looping
Always add a retry cool-down in your Kafka consumer fetch loop when broker connections fail. This avoids unthrottled loop execution, CPU spikes, and log flooding:
```go
km, err := c.reader.FetchMessage(ctx)
if err != nil {
    if ctx.Err() != nil {
        return // Graceful shutdown
    }
    c.log.Errorw("fetch message failed", "error", err)
    time.Sleep(1 * time.Second) // Cooldown block
    continue
}
```
