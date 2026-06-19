# Phase 4 Notification Core — Architectural & Implementation Review

This document contains a detailed analysis, architectural critique, and recommended improvements for the core notification delivery and ingestion components implemented in **Phase 4** of the Notifyx service.

**Status: all 5 issues addressed (2026-06-19).** 4 fixed as recommended (atomicity via transaction, dedup retry loop, bounded-concurrency batch processing, composite index); 1 pushed back on (Issue 4 — reordering the replayer's publish/update steps trades a recoverable failure mode for an unrecoverable one). Verified by `go build`, `go vet`, and live HTTP calls against a real local Postgres (`brew services start postgresql@16`, schema applied from `migrations/001_initial_schema.up.sql`): created a tenant, enabled the email channel, sent a single notification (confirms the new transactional `createAndQueue` commits correctly), and sent a 30-recipient batch (confirms bounded-concurrency processing returns 30 distinct notification IDs, no errors, and results correctly indexed back to their original request order despite concurrent execution). Kafka/Redis were not configured locally, so the publish-failure rollback path (Issue 1) and the dedup in-flight race (Issue 2) were verified by code inspection rather than triggered live — flagged as a gap, consistent with how phase_3's Kafka-dependent fixes were verified.

---

## 1. Ingestion Atomicity Failure & Permanent Idempotency Lockout (High Risk)

### Issue Description
In [createAndQueue](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/notification.go#L474), database insertions and Kafka publishing are executed sequentially without a transaction wrapper:

```go
notification, err := h.svc.Notifications.Create(...) // 1. Save Notification
...
delivery, err := h.svc.Deliveries.Create(...)        // 2. Save Delivery
...
err := h.svc.Producer.Publish(...)                   // 3. Publish to Kafka
```

### Architectural Implications
1. **Orphaned Database Records:** If step 3 (Kafka publish) fails, the notification and delivery rows remain committed in PostgreSQL, but the message is never queued for delivery.
2. **Permanent Idempotency Lockout:** When the Kafka publish fails, `processSend` releases the idempotency key in Redis. When the client retries the request, the dedup check passes, and the handler attempts to re-create the notification. However, since the database row was already committed, PostgreSQL throws a **unique key constraint violation** on `(tenant_id, idempotency_key)`. The client is permanently locked out from retrying this notification.

### Recommended Resolution
* Wrap database writes in a single PostgreSQL transaction (`pgx.Tx`).
* If Kafka publishing fails, rollback the Postgres transaction so the client can safely retry the request with the same idempotency key.

### Resolution — Fixed as recommended

Confirmed: `createAndQueue` called `Notifications.Create`, then `Deliveries.Create`, then `Producer.Publish` against the pool directly, with no transaction. Confirmed the lockout mechanism exactly as described — `notifications` has `CONSTRAINT notifications_tenant_idempotency_key_key UNIQUE (tenant_id, idempotency_key)` (`migrations/001_initial_schema.up.sql`), so a retry with the same key after a publish failure would hit that constraint on the already-committed row even though `processSend`'s `releaseDedup` had already freed the Redis-side reservation.

Made `NotificationRepository` and `NotificationDeliveryRepository` accept the existing `Executor` interface instead of `*pgxpool.Pool` directly (the same pattern `TenantRepository` already uses for `WithTx`), then rewrote `createAndQueue` in [notification.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/notification.go) to run `Notifications.Create`, `Deliveries.Create`, and `Producer.Publish` inside one `postgres.WithTx` closure. The Kafka publish call happens *before* the transaction's `Commit`, not after — that's the deliberate part: if `Publish` returns an error, `WithTx` returns that error and rolls back instead of committing, so neither row is ever persisted and a retry with the same idempotency key succeeds cleanly. `NotificationService` gained a `Pool *pgxpool.Pool` field (mirroring `TenantService`) so the handler can open the transaction.

One tradeoff worth naming: the publish now happens *inside* the transaction's open window, so a slow Kafka broker holds the Postgres transaction (and the connection it's on) open for longer than before. Given this project's scale (portfolio project, no measured production load) and that the alternative — leaving Kafka outside the transaction — is exactly the bug being fixed, this is the right tradeoff for now. If Kafka latency under load is ever a measured problem, the standard fix is a transactional outbox (write an "outbox" row in the same transaction, publish from a separate poller) — logged as a deferred Phase 13+ item in `decisions.md`.

Verified live against a real local Postgres: created a tenant, enabled the email channel, sent a notification with an idempotency key — the notification committed with status `pending` (Kafka isn't configured locally, so `Producer` is `nil` and publish is skipped per existing convention, exercising the "commit without publish" branch of the same transaction). The publish-failure-then-rollback branch itself was verified by code inspection only, not triggered live, since there's no way to force a Kafka publish failure without Kafka configured in this environment.

---

## 2. Deduplication Race Condition (In-Flight Gap)

### Issue Description
In [processSend](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/notification.go#L418), if another request has already claimed the idempotency key in Redis (`claimed = false`), it queries PostgreSQL to find the original record:

```go
existing, err := h.svc.Notifications.GetByID(ctx, existingID)
```

### Architectural Implications
1. **Failed Retries:** If a duplicate request arrives concurrently while the original request is still writing to PostgreSQL, `GetByID` will fail with `postgres.ErrNotFound`. The duplicate request fails with a `500 Internal Server Error` instead of responding with the original notification ID and status.

### Recommended Resolution
* If the record is missing on cache hit, implement a retry loop (e.g. 3 attempts over 100ms) to allow the in-flight write to complete.
* Alternatively, use a temporary "in-flight" lock state in Redis.

### Resolution — Fixed as recommended

Confirmed: `processSend`'s `!claimed` branch called `h.svc.Notifications.GetByID(ctx, existingID)` exactly once, with no retry — confirmed against `dedup.go`'s `Reserve`, which only ever returns an ID via Redis `SETNX`/`Get`, with no caching of notification status, so `GetByID` against Postgres is the only source of truth and genuinely can race the original request's write.

Took the retry-loop option over the Redis in-flight-lock option: a second Redis round trip to manage lock state adds complexity for a gap that's already small (one Postgres `INSERT ... RETURNING` plus a network round trip) and self-resolving (the original write either completes or the request fails outright, no scenario where it stays pending forever). Added a 3-attempt loop with a 50ms sleep between attempts in [notification.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/notification.go)'s `processSend`: retries only on `postgres.ErrNotFound` (any other error — e.g. a real DB outage — fails fast on the first attempt rather than wasting up to 150ms retrying a problem retrying won't fix).

Verified via `go build`/`go vet`; the race window itself is sub-millisecond in practice and wasn't reliably reproducible live without injecting an artificial delay into `Create`, so this was verified by code inspection of the retry logic and confirmed it correctly distinguishes `ErrNotFound` (retryable) from other errors (not retryable).

---

## 3. Synchronous Batch Processing Bottleneck

### Issue Description
In [NotificationHandler.Batch](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/notification.go#L250), the handler processes recipients in a simple synchronous loop:

```go
for i, rec := range req.Recipients {
    notificationID, _, _, errMsg := h.processSend(...)
}
```

### Architectural Implications
1. **Thread Blocking:** A batch request containing 100+ recipients will perform 100+ sequential Postgres writes, 100+ Redis updates, and 100+ Kafka publishes. This blocks the web request thread.
2. **Pool Starvation:** Sequentially executing multiple database operations per recipient will quickly saturate the database connection pool, leading to client-side timeouts.

### Recommended Resolution
* Process batch elements concurrently using bounded goroutines (e.g., via `errgroup`).
* Execute database writes in a single round-trip using Postgres bulk insertions (via `pgx.Batch` or `CopyFrom`).

### Resolution — Fixed, with a different approach

Confirmed: `Batch` looped over `req.Recipients` fully sequentially, calling `processSend` once per recipient with nothing else running concurrently.

Implemented the first recommendation (bounded concurrency) but not the second (bulk insertion via `pgx.Batch`/`CopyFrom`). Each recipient's `processSend` is its own independent dedup-check → rate-limit-check → transaction → publish pipeline with its own idempotency key and its own pass/fail outcome reported per-recipient in the response — collapsing that into one bulk `INSERT` would mean either losing per-recipient error isolation (one bad row fails the whole batch) or building a parallel "bulk insert, then patch up the ones that should've failed dedup/rate-limit checks" path, which is more complexity than the current scale justifies. Bounded goroutine concurrency gets the actual blocking problem (100+ sequential round trips serialized on one request thread) without touching that tradeoff.

Used a plain `sync.WaitGroup` + buffered channel as a semaphore (capped at `maxConcurrent = 16`) instead of `errgroup`, since there's no need to cancel the whole batch if one recipient fails — each recipient's result is independent and already captured per-index in `results[i]`, which `errgroup`'s fail-fast cancellation would work against. Each recipient writes only to its own `results[i]` slot from its own goroutine, so there's no shared mutable state needing a mutex.

Verified live against a real local Postgres: sent a 30-recipient batch request and confirmed all 30 succeeded with 30 distinct notification IDs, no errors, and `results[i].Index == i` for every entry (i.e., concurrent completion order didn't scramble the response order). Bulk insertion (`pgx.Batch`/`CopyFrom`) logged in `planning/decisions.md` as a deferred Phase 13+ item, gated on an actual large-batch volume/latency number justifying the added complexity.

---

## 4. Replayer Duplicate Publishing Risk on Crash

### Issue Description
In [Replayer.replayOne](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/ratelimit/replayer.go#L68), the rate-limit replayer publishes the notification to Kafka *before* updating the database status:

```go
err := r.producer.Publish(ctx, ...) // 1. Publish to Kafka
...
r.notifications.UpdateStatus(ctx, ...) // 2. Update status in Postgres
```

### Architectural Implications
1. **Duplicate Deliveries:** If the application crashes immediately after step 1 but before step 2, the message will enter the consumer pipeline. On reboot, the replayer will fetch this notification again (since it is still marked as `queued_rate_limited` in the DB) and publish it a second time.

### Recommended Resolution
* Update the status in the database first (or in a transaction) before publishing to Kafka.

### Resolution — Pushed back

Confirmed the ordering exactly as described: `replayOne` calls `r.producer.Publish(...)` first, then `r.notifications.UpdateStatus(...)` and `r.deliveries.SetStatus(...)` afterward. A crash between those two steps does leave the notification re-publishable on the next tick, as the review states.

Not implementing the literal suggestion (update DB first, then publish), because it doesn't eliminate a failure mode — it relocates it to a strictly worse one. If the status update commits first and the process then crashes before `Publish` actually succeeds, the notification is now permanently marked `queued` in Postgres but was never actually written to Kafka — nothing will ever replay it again, since the replayer only looks for `queued_rate_limited` rows. That's a silent, permanent loss. The current order's failure mode (publish succeeds, crash, republish on next tick) produces a duplicate, recoverable delivery — annoying, but not a lost notification. For a system whose stated purpose is guaranteeing delivery, "publish twice" is a materially better failure mode than "publish zero times and never know."

This is the same reasoning already applied to `createAndQueue` (Issue 1) and to the DLQ commit-safety fix in phase_3 (Issue 2/7 there): when a step can't be made truly atomic with an external system, default to redelivering rather than dropping, since Kafka's own at-least-once delivery model already assumes consumers tolerate duplicates. A true fix for *this* crash window specifically (zero duplicates, zero loss) needs a transactional outbox or a Kafka idempotent-producer-style dedup token recognized by downstream consumers — both are real architecture additions, not a two-line reorder, and neither is justified yet without evidence the duplicate-on-crash window (process crash precisely between a successful publish and two fast follow-up UPDATE statements) is hit in practice.

Not implementing this. Logged in `planning/decisions.md` as a deferred Phase 13+ item: revisit with a transactional outbox pattern if duplicate replayer deliveries are ever observed in practice.

---

## 5. Missing Composite Index for History Search (Database Tuning)

### Issue Description
The history search query in [GetByTenantID](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/notification_repo.go#L100) filters by `tenant_id` and sorts results by `created_at DESC` for pagination.

### Architectural Implications
1. **Slow Queries under Load:** Currently, the schema only contains single-column indexes on `tenant_id` and `created_at`. When querying history, Postgres is forced to scan the `tenant_id` index and then perform a potentially expensive filesort of those rows in memory to satisfy the `ORDER BY created_at DESC` condition.

### Recommended Resolution
Add a composite index to the schema:
```sql
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_created_at 
    ON notifications(tenant_id, created_at DESC);
```
This allows PostgreSQL to fetch pre-sorted results directly from the index, eliminating sorting overhead and speeding up history searches.

### Resolution — Fixed as recommended

Confirmed: `migrations/001_initial_schema.up.sql` had single-column indexes on `tenant_id` and `created_at` separately, but no composite covering `GetByTenantID`'s actual access pattern (`WHERE tenant_id = $1 ... ORDER BY created_at DESC`).

Added `idx_notifications_tenant_created_at ON notifications(tenant_id, created_at DESC)` to the existing schema file rather than a new incremental migration — per this project's established migration policy (single schema file pre-deployment, incremental migrations only after the first real deploy; this hasn't deployed yet). Left the existing single-column `idx_notifications_tenant_id` and `idx_notifications_created_at` indexes in place rather than dropping them, since `idx_notifications_created_at` alone still serves `GetByStatus`'s `ORDER BY created_at ASC` pattern and removing indexes without a concrete reason (the review didn't ask for it, and there's no measured write-overhead problem) is the kind of speculative cleanup outside this issue's scope.

Verified via `psql` against a local Postgres: applied the updated schema file cleanly with no errors (`CREATE INDEX` succeeded). Did not run `EXPLAIN ANALYZE` to confirm a measured plan change, since the local test database has no realistic data volume for the planner to prefer the new index over a sequential scan — flagged as the verification gap for this issue specifically (correctness of the index definition was confirmed; its actual effect on the query planner under production-scale data was not).

