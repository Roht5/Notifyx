# Notifyx — Human & AI Developer Guide

This document serves as the central engineering manual for Notifyx. It outlines the codebase architecture, mandatory coding patterns, data safety constraints, and local CLI operations. It is designed to orient both **human software engineers** and **AI coding assistants** (e.g., Claude Code).

---

## 🗺️ Architectural Core Rules

All development (across all phases) must strictly adhere to these architectural boundaries:

### 1. Multi-Tenant Scoping (Non-Negotiable)
* Every database table (except the `tenants` table itself) MUST have a `tenant_id` column.
* Every single SQL select, update, or delete query must be explicitly scoped using `WHERE tenant_id = $1` to guarantee absolute data isolation.
* Unauthenticated database access is prohibited. All endpoints (except super-admin tenant setup and `/health`) must pass through the `Auth` middleware, which injects the `*domain.Tenant` object into Echo's request context.
* Use the [RequireTenant](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/common.go#L33) helper to safely retrieve the tenant context:
  ```go
  tenant, err := RequireTenant(c)
  if err != nil {
      return err // Safe type cast, no panic risk
  }
  ```

### 2. Transactional Database Modifications (Atomicity)
* Any HTTP request or service function that executes more than one database modification query (e.g., inserts/updates/deletes) MUST run inside a single database transaction (`pgx.Tx`).
* **Example (Tenant creation):** Wrapping tenant insertion and API key hash insertion in a transaction ensures no orphaned tenants are created.
* **Example (Batch updates):** Updating multiple settings or rate limits in a loop must be wrapped in a transaction so that if one fails, the database rolls back to a clean state.

### 3. Redis Key Naming & Cluster Compatibility
* Notifyx supports clustered Redis deployments (such as Upstash or AWS ElastiCache).
* Multi-key commands (like Lua scripts checking multiple keys) will crash with a `CROSSSLOT` error if the keys hash to different slots.
* **Rule:** You MUST wrap the `tenant_id` inside **hash tags** `{...}` in all Redis key formatting strings. This forces Redis to hash only the tenant ID, guaranteeing all keys for a single tenant land on the same node slot:
  * **Channel Rate Limit Key:** `rate:{<tenant_id>}:<channel>`
  * **Global Rate Limit Key:** `rate:{<tenant_id>}:global`
  * **Idempotency Key:** `dedup:{<hashed_idempotency_key>}`

---

## 🗄️ Database & Schema Conventions

### 1. Checking `rows.Err()` on Iterations
When querying multiple rows using `rows.Next()`, you MUST check `rows.Err()` immediately after the loop terminates. If a connection issue or timeout occurs mid-query, the loop will exit early silently.
```go
for rows.Next() {
    // scan rows
}
if err := rows.Err(); err != nil {
    return nil, fmt.Errorf("iterate rows failed: %w", err)
}
```

### 2. Indexes & Constraints
* **Redundant Index Guard:** Do not manually create an index on columns marked as `UNIQUE` (e.g., `key_hash` or composite unique constraints). PostgreSQL automatically builds unique indexes for these constraints; manual indexes waste writes and disk space.
* **Ordered Queries Optimization:** For tables with pagination ordered by date (like history logs), use a composite index on `(tenant_id, created_at DESC)` to allow Postgres to perform index scans rather than expensive in-memory filesorts.

---

## ✉️ Event-Driven Kafka Guidelines

### 1. Reliable Consumer Handoff
* Do not commit a message offset if the event was aborted or if writing to the Dead Letter Queue (DLQ) failed.
* In [Consumer.Run](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/kafka/consumers/consumer.go#L90), only call `CommitMessages` once the message has either been delivered successfully or safely persisted to `dlq_messages` in PostgreSQL.
* Modify the handler to return an abort state on context cancellation during graceful shutdowns so the offset is **not** committed prematurely.

### 2. Throttled Error Paths (Busy Loop Guard)
* When polling Kafka via `FetchMessage`, any connection or auth error must block the thread for a cooldown duration (e.g. `time.Sleep(1 * time.Second)`) before executing `continue`. This prevents unthrottled infinite loops that spike CPU to 100% and flood logging servers.

### 3. Producer Durability
* Set `RequiredAcks` to `kafka.RequireAll` (equivalent to `acks=all`) in [producer.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/kafka/producer/producer.go) to ensure all in-sync replicas acknowledge a write before returning success.

---

## 🛡️ Telemetry & Logging Patterns

### 1. Error Formatting in Middleware
* Middlewares (like [RequestLogger](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/middleware/request_logger.go)) must handle Echo router-level errors (such as 404s/405s) by calling `c.Error(err)` to force-commit the correct HTTP status code before reading `res.Status` and logging.

### 2. Log Levels & Verbosity
* Use structured logging (`log.Infow`, `log.Errorw`) and avoid formatting strings in log statements.
* Log levels are configurable via the `LOG_LEVEL` environment variable at startup, which overrides default APP_ENV behaviors.

---

## 🛠️ CLI Operations Reference Sheet

Use the following commands inside the `backend/` directory for local development:

### 1. Running the Application
```bash
# Load environment config and run the server
go run cmd/server/main.go
```

### 2. Database Migrations
Migrations are managed via the `golang-migrate` driver:
```bash
# Apply all up migrations
migrate -path migrations -database "postgres://user:pass@localhost:5432/notifyx?sslmode=disable" up

# Rollback the last migration
migrate -path migrations -database "postgres://user:pass@localhost:5432/notifyx?sslmode=disable" down 1
```

### 3. Verification & Testing
```bash
# Run unit and integration tests
go test -v ./...

# Format codebase
go fmt ./...
```
