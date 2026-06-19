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

**Rate limit check in the handler/service layer, not as Echo middleware (deviation from task-breakdown wording)**
A generic middleware can only allow or reject a request — it can't decide to persist a rate-limited notification as `queued_rate_limited` and skip publishing instead of erroring, since that requires writing to Postgres before the handler's business logic even runs. The check lives in `NotificationHandler.processSend` instead, with the same nil-checked degrade-gracefully pattern used for the Kafka producer.

**Single `001_initial_schema` migration, not one file per table (deviation from task-breakdown wording, by request)**
The original plan called for 9 separate per-table migrations; two more (010, 011) got bolted on later fixing bugs found in review (missing `updated_at` on templates, a global instead of per-tenant unique constraint on `idempotency_key`). Since none of these had ever run against a real database — no deployment, no `.env`, nothing — there was no "already shipped, never edit a migration" constraint to respect, so all 11 files were consolidated into one `001_initial_schema.up/down.sql` with both fixes baked in from the start. Verified locally: spun up Postgres via Homebrew, ran the real `golang-migrate` up → down → up round-trip against a scratch database, confirmed the schema (including the per-tenant unique constraint and the `updated_at` column) and a clean drop, then tore the scratch DB and service back down.

**`pgcrypto` extension + two redundant indexes removed (code review fixes)**
A second code review pass (Codex) flagged: (1) `gen_random_uuid()` needs `pgcrypto` pre-PG13 — added `CREATE EXTENSION IF NOT EXISTS pgcrypto` as free insurance, though Render's free tier and the local Homebrew Postgres used for verification are both well past 13, so this wasn't an active risk; (2) `api_keys.key_hash` had both an inline `UNIQUE` constraint *and* a separate `CREATE INDEX` on the same column — the second was a fully redundant B-tree, pure write overhead, removed; (3) `notifications.idempotency_key` had a standalone index that nothing queries (dedup is Redis-based, `SETNX`) — removed rather than just narrowed to partial, since the composite `UNIQUE(tenant_id, idempotency_key)` already covers every tenant-scoped lookup the app does. All three re-verified with a real up→down→up round-trip and `\d` inspection.

**Domain entities keep `json` tags rather than moving to dedicated DTOs (disagreement with a code review suggestion)**
Codex's review flagged `json` tags on `domain.Notification` as a Clean Architecture violation — domain entities shouldn't know about transport serialization; the textbook fix is a DTO per entity, mapped explicitly in the handler layer. Agreed the underlying inconsistency was real (some domain structs had tags, most didn't — `Tenant`, `TenantChannel`, `TenantRateLimit`, `APIKey`, `NotificationTemplate`, `DLQMessage` were all still using Go's default field-name JSON encoding, i.e. capitalized keys, while `Notification`/`NotificationDelivery` used snake_case). Disagreed with the proposed fix: a full DTO-per-entity layer is real ceremony — a parallel struct and an explicit mapping function for every domain type returned by any endpoint, growing with every remaining phase — and the payoff (decoupling domain from transport) doesn't clearly outweigh that cost for a solo, single-binary portfolio project with no other consumer of the domain layer. Resolved the actual inconsistency the lighter way instead: added `json` tags directly to the remaining untagged domain structs, so every API response is consistently snake_case. `APIKey.KeyHash` got `json:"-"` rather than a tag, since nothing should ever return a key hash over the API.

**Request logger now force-commits the status code via `c.Error` before logging (bug fix)**
`RequestLogger` read `res.Status` immediately after `next(c)` returned, which is correct for every handler in this codebase (they all call `c.JSON`/`c.NoContent` themselves before returning) — but wrong for anything Echo's own router generates without ever reaching a handler, like 404s and 405s. Echo's `Echo.ServeHTTP` only invokes `HTTPErrorHandler` (which actually writes the status+body) *after* the full middleware chain returns — confirmed by reading the vendored `echo@v4.15.4` source directly rather than taking the review's claim on faith. So a bare unhandled error reaching `RequestLogger` would log whatever `res.Status` happened to be before any write (its zero value, 200) with no indication anything failed. Fixed by calling `c.Error(err)` ourselves when `err != nil` before reading the status — confirmed safe by checking `DefaultHTTPErrorHandler`, which no-ops if the response is already committed. Verified live: hit a nonexistent route against a running instance, log line went from would-have-been `INFO`/200/no error to `ERROR`/404/`"error": "code=404, message=Not Found"`.

**Explicit Postgres connection pool limits + fail-fast config validation (hardening, not a real bug)**
Codex claimed pgxpool defaults to ~90-100 connections; checked the vendored source — the actual default is `max(4, runtime.NumCPU())`, not that. So this wasn't the live-wire risk the review implied. Still added `DB_MAX_CONNS`/`DB_MIN_CONNS` env vars (defaults 10/0) wired into `pgxpool.Config.MaxConns/MinConns` in `NewPool`, since an implicit, CPU-count-derived pool size is still less predictable than an explicit one, and free-tier Postgres plans do cap total connections low. `MinConns` defaults to 0 (not eagerly holding idle connections against that cap) rather than the review's suggested 2 — this app is mostly idle between requests, and Render's free web service cold-starts anyway, so pre-warming connections buys little. Also added `Config.Validate()`, called from `Load()`, checking `DATABASE_URL` is set and `APP_ENV` is a recognized value — fails at startup with a clear message instead of a confusing error later inside pool init or migrations.

**Migrations still run from `main()` on startup, not decoupled into a separate deploy step (acknowledged but deferred to Phase 13)**
Codex flagged that running `golang-migrate` inside the app's own startup path races under horizontal scaling — multiple instances starting concurrently all hit the advisory lock, and slow ones can time out a deploy. Real concern in general, but checked against this project's actual plan: `requirements/tech-stack.md` specifies "Render (1 Web Service + 1 PostgreSQL)" and `non-functional-requirements.md` describes Notifyx as "single deployable service" with no horizontal scaling planned — so the concurrent-instance race this describes doesn't apply to Notifyx's documented deployment topology. Decoupling migrations now, with no Dockerfile/render.yaml/pre-deploy pipeline yet to attach it to, would be solving a problem this project doesn't have yet. Tracked as a Phase 13 checklist item instead: when the actual Render config is built, point its Pre-Deploy Command at a migration-only run and stop running migrations inside `main()`.

**Stale `bcrypt` comment fixed to say SHA-256 (doc bug)**
`domain.APIKey`'s comment said "we store only the bcrypt hash," left over from before the SHA-256 decision was made (SHA-256 chosen specifically because it's deterministic, so `WHERE key_hash = ?` works — bcrypt salts itself per call and can't be looked up this way). The actual implementation (`apikey.go`) was always correct; only the comment was wrong. Caught in code review (Codex).

**`LOG_LEVEL` env var added to override the Env-derived default verbosity (operability improvement)**
Previously log level was hardcoded to the `APP_ENV` choice — Info in production, Debug in development — with no way to turn verbosity up or down without a redeploy. Added an optional `LOG_LEVEL` (debug/info/warn/error/...) that overrides whichever default `APP_ENV` would have picked; `logger.New` fails fast on an unrecognized value rather than silently falling back. Verified live: `LOG_LEVEL=warn` suppressed the INFO-level "http request" line for a successful `/health` call but still logged the ERROR-level line for a 404.

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

**Global cap moved from `tenant_rate_limits` to `tenants` (bug fix)**
The original schema stored `global_cap` as a column on the per-channel `tenant_rate_limits` row. Since a tenant gets one row per channel it configures, a tenant with custom limits on email and SMS could end up with two different "global cap" values stored — and `Limiter.resolveLimits` would enforce whichever one happened to belong to the channel being checked, against the *same* Redis global-window key. Caught in code review (Codex). Fixed by adding `tenants.global_rate_cap` (one value per tenant, matching what "global" actually means) and dropping `global_cap` from `tenant_rate_limits` entirely. `PUT /tenants/:id/rate-limits` now takes `global_cap` once at the top level instead of once per channel entry. Verified against a real local Postgres and the live HTTP endpoint (create tenant → check default 300 → update global_cap to 500 → confirm it stuck).

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

---

## Phase 2/3 Code Review (Codex) — 2026-06-19

**`Executor` interface + `WithTx` helper for transactional multi-write handlers (bug fix)**
Tenant creation (tenant + first API key) and the channel/rate-limit bulk-update loops each issued multiple independent writes with no transaction, so a mid-loop failure left partial state committed. Added a small `Executor` interface (`Exec`/`Query`/`QueryRow`) that both `*pgxpool.Pool` and `pgx.Tx` satisfy, plus `WithTx(ctx, pool, fn)`, so the same repository code runs against either. `tenant.go`'s `Create`, `UpdateChannels`, and `UpdateRateLimits` now wrap their multi-write sequences in `WithTx`. Verified against a real local Postgres and live HTTP calls.

**API key rotation/revocation endpoints added (security gap fix)**
The only way to invalidate a compromised API key was deleting the whole tenant (cascades to all their data). `APIKeyRepository` already had unused `Create`/`Delete`/`GetByTenantID` methods from an earlier phase, never wired to a route. Added `POST/GET /tenants/:id/keys` and `DELETE /tenants/:id/keys/:key_id`. `DeleteKey` doesn't verify the key belongs to `:id` first — `api_keys.id` is globally unique and these are already unauthenticated super-admin routes, so `:id` is just URL shape, not an authorization boundary. Verified live: create → rotate in a second key → list shows 2 → revoke → list shows 1.

**Redis caching for `key_hash → tenant` auth lookups: deferred, not implemented**
Every authenticated request hits Postgres via `GetTenantByKeyHash` — won't scale to high send throughput. Not building it now: no real production load to size a TTL against, and correct invalidation (on tenant delete *and* the key revocation endpoint just added) is real complexity for a problem that doesn't exist yet. Tracked as a Phase 13+ checklist item: cache in Redis with a 5–15 min TTL, invalidate on delete/revoke/update.

**`TenantFromContext` made a safe assertion; added `RequireTenant` helper (bug fix, different shape than reviewer suggested)**
`TenantFromContext` did an unchecked type assertion (`c.Get("tenant").(*domain.Tenant)`) that panics if a route is ever registered without the auth middleware. The reviewer's suggested fix (`(*domain.Tenant, bool)` return) only moves the panic to the next line, where every caller immediately dereferences the tenant. Instead made the assertion safe (returns nil) and added `RequireTenant(c) (*domain.Tenant, error)`, which writes a clean 500 instead of panicking. All 4 call sites in `notification.go` updated. Verified live.

**`rows.Err()` checks added after every `for rows.Next()` loop (data-integrity bug fix)**
A connection drop mid-scan makes `rows.Next()` return `false` with no error visible to the caller, silently truncating result sets. Fixed in the 4 files the review flagged (`tenant_repo.go`, `api_key_repo.go`, `tenant_channel_repo.go`, `tenant_rate_limit_repo.go`) plus 2 more found by the same pattern while converting these files to the `Executor` interface (`notification_repo.go`, `notification_delivery_repo.go`). Left `TenantRateLimitRepository.GetByChannel`'s separate bug (swallows all `QueryRow` errors as "use default", not just `ErrNoRows`) unfixed — real but out of scope for this pass.

**CORS middleware added (dashboard blocker fix)**
No CORS config existed; the Phase 12 Flutter Web dashboard would have every fetch silently dropped by the browser. Added Echo's CORS middleware with `AllowOrigins: []string{"*"}` — acceptable since the dashboard is unauthenticated (no cookies/credentials in play); tighten to a specific origin once Phase 12 ships a deployed dashboard. Verified live via curl.

**Kafka partition key changed from `msg.Priority` to `msg.NotificationID` (bug fix, not the reviewer's literal suggestion)**
Keying on priority (3-4 distinct values) sent every message of one priority to the same partition regardless of partition count — worse than no key, since it killed parallelism across consumers without providing any actual priority preemption (Kafka has no mechanism for a consumer to jump a "critical" partition ahead of a "normal" one anyway). Reviewer suggested splitting into one topic per priority class plus weighted polling; rejected as premature — multiplies topic count 4x with no priority-aware consumer scheduling yet built to use it. Switched the key to `msg.NotificationID` instead: high-cardinality (restores partition spread) while still routing one notification's messages to the same partition for ordering. True priority preemption remains unsolved; tracked as a Phase 13+ item if priority semantics ever become a real product requirement.

**Kafka consumer commit-safety: `handleWithRetry` returns `bool`, `Run()` only commits on success (data-loss bug fix)**
The consumer committed the offset unconditionally after `handleWithRetry`, regardless of whether the DLQ publish (on retry exhaustion) succeeded, or whether a shutdown signal interrupted a retry backoff mid-flight. Both cases silently dropped a message with no record anywhere. Changed `handleWithRetry` to return `bool` ("safe to commit"): `true` on handler success, on an unprocessable/malformed message, or after a confirmed DLQ publish; `false` if the DLQ publish itself failed or `ctx.Done()` fired during a retry wait. `Run()` only calls `CommitMessages` on `true`. This single mechanism fixes both the "silent DLQ publish failure" and "premature commit on shutdown" issues the review raised separately.

**`Message.LastError` field carries the real failure reason into the DLQ (bug fix, JSON field instead of Kafka headers)**
The DLQ consumer persisted a hardcoded `"exhausted delivery retries"` string, discarding the actual provider error (e.g. a 401 from Resend). Reviewer suggested Kafka message headers; used a JSON field on the existing `Message` struct instead, since the whole message is already JSON-decoded on both ends — no header encode/decode code needed. `handleWithRetry` sets `msg.LastError` before the DLQ publish; `dlq.go` reads it instead of the hardcoded string.

**`fetchErrorBackoff` (2s) added before retrying a failed `FetchMessage` (bug fix)**
A persistent broker/network error looped with no delay, spiking CPU and flooding logs. Added a 2-second backoff via `select` on a timer or `ctx.Done()` (not a bare `time.Sleep`, so shutdown isn't delayed by the backoff window).

**`RequiredAcks` changed from `RequireOne` to `RequireAll` in the Kafka producer (bug fix)**
`RequireOne` only waits for the partition leader to ack, risking data loss if the leader fails before replicating. Changed to `RequireAll` for full in-sync-replica acknowledgment before a publish is considered successful.

**Kafka auto-commit (`CommitInterval > 0`) rejected, not implemented**
Reviewer flagged synchronous per-offset commits as a throughput bottleneck and suggested auto-commit on a timer. Rejected: auto-commit has no concept of "this message wasn't safely handled yet" — it would commit on a fixed schedule regardless of `handleWithRetry`'s return value, silently undoing the commit-safety fix above and reintroducing the exact data-loss bug it fixed. No throughput measurement exists yet to justify the tradeoff. If ever needed, the right fix is manual batched commits that still respect the safety signal, not blanket auto-commit. Tracked as a Phase 13+ item, gated on having an actual throughput number.

## Phase 4 Notification Core Review (Codex) — 2026-06-19

**`NotificationRepository`/`NotificationDeliveryRepository` converted to `Executor`, `createAndQueue` wrapped in `WithTx` (atomicity + idempotency-lockout bug fix)**
`createAndQueue` persisted the notification and delivery rows, then published to Kafka, with no transaction tying the three together. A publish failure left both rows committed; `processSend` released the Redis dedup reservation on that failure, so a client retry with the same idempotency key hit the `(tenant_id, idempotency_key)` unique constraint on the orphaned row and was permanently locked out. Converted both repositories to accept the existing `Executor` interface (the same pattern `TenantRepository` already uses), gave `NotificationService` a `Pool` field, and rewrote `createAndQueue` to run create-notification, create-delivery, and Kafka publish inside one `postgres.WithTx` closure — the publish happens before commit, so a publish failure rolls back both rows and the idempotency key is free for a clean retry. Verified live against a local Postgres (notification creates and commits correctly with Kafka unconfigured, exercising the no-publish commit path); the publish-failure rollback path itself was verified by code inspection only, since Kafka isn't configured locally.

**Dedup in-flight race fixed with a 3-attempt retry loop (50ms apart), not a Redis lock state**
On a dedup cache hit (`!claimed`), `processSend` called `Notifications.GetByID` once with no retry — if the original request's Postgres write hadn't committed yet, the duplicate request got a `500` instead of the original notification's status. Added a 3-attempt retry loop in `processSend`, retrying only on `postgres.ErrNotFound` (other errors fail fast). Chose this over a temporary Redis "in-flight" lock state — the race window is small and self-resolving, and a second piece of Redis state to manage didn't seem justified for it. Verified via code inspection of the retry/error-type logic; the race itself wasn't reliably reproducible live without injecting an artificial delay.

**`NotificationHandler.Batch` processes recipients with bounded goroutine concurrency (`maxConcurrent = 16`), not bulk Postgres inserts**
`Batch` looped over recipients fully sequentially, serializing 100+ Postgres writes / Kafka publishes on one request thread and risking DB pool starvation. Added a `sync.WaitGroup` + buffered-channel semaphore capped at 16 concurrent `processSend` calls, each writing only to its own `results[i]` slot. Did not implement the bulk-insert (`pgx.Batch`/`CopyFrom`) half of the suggestion — each recipient has its own independent dedup/rate-limit/publish outcome that needs reporting per-recipient, and collapsing that into one bulk insert trades away that isolation for a savings not yet justified at this project's scale. Used a plain semaphore instead of `errgroup` since one recipient's failure shouldn't cancel the rest of the batch. Verified live: 30-recipient batch returned 30 distinct notification IDs, no errors, correct index-to-result mapping despite concurrent completion order.

**Replayer publish-then-update ordering kept as-is; reordering rejected**
The review flagged `Replayer.replayOne` publishing to Kafka before updating Postgres status as a duplicate-delivery risk on a crash between the two steps, and suggested updating the DB first. Rejected: doing the DB update first just relocates the failure mode to a worse one — a crash after the update but before the publish actually lands leaves the notification permanently marked `queued` with no way to ever notice it was never published, since the replayer only looks for `queued_rate_limited` rows. The current order's failure mode (duplicate redelivery) is recoverable; the suggested order's failure mode (silent permanent loss) isn't — same reasoning already applied to the Issue 1 fix above and to the DLQ commit-safety fix from phase_3. A real fix for the crash window itself needs a transactional outbox or an idempotency token recognized by downstream consumers; tracked as a Phase 13+ item, gated on observing actual duplicate deliveries in practice.

**Composite index `idx_notifications_tenant_created_at` added to the schema (DB tuning)**
`GetByTenantID` (notification history) filters by `tenant_id` and sorts by `created_at DESC`, but only single-column indexes existed on each separately, forcing a filesort. Added `(tenant_id, created_at DESC)` directly to `migrations/001_initial_schema.up.sql` (project hasn't deployed yet, so the schema is still a single pre-deployment file per existing migration policy) without dropping the existing single-column indexes, since `idx_notifications_created_at` still serves `GetByStatus`'s separate access pattern. Verified the migration applies cleanly on a local Postgres; did not run `EXPLAIN ANALYZE` to confirm a plan change, since the local test DB has no realistic data volume for the planner to prefer it over a sequential scan.

## Phase 5 Rate Limiting & Deduplication Review (Codex) — 2026-06-19

**Rate-limit Redis keys hash-tagged with `{tenantID}` (clustering bug fix)**
`Limiter.Allow` built its per-channel and global keys as plain `rate:<tenantID>:<channel>` / `rate:<tenantID>:global`, passed together into one Lua script — on a clustered Redis these two strings hash to different slots and the script aborts with `CROSSSLOT`. Wrapped `tenantID` in `{}` in both key formats so Redis only hashes the tag, guaranteeing both land on the same slot. A no-op on the single-node Redis this project currently runs against, but free insurance against ever moving to a clustered instance. Verified live: keys read `rate:{tenantID}:email` / `rate:{tenantID}:global`, and the sliding-window cap still correctly blocked requests past the limit.

**Dedup `Reserve` made atomic via a single Lua script, dedup keys SHA-256 hashed (latency + memory-bloat fixes)**
`Reserve` did a `SetNX` then a separate `Get` on every duplicate hit — two round trips where one suffices, costly on a WAN-hop Redis like Upstash. Replaced with one Lua script (`reserveScript`) that claims-or-reads in a single round trip. Separately, `redisKey` built the Redis key directly from the client-supplied idempotency key with no length limit, letting an oversized key bloat Redis memory; now SHA-256-hashed to a fixed 64-hex-char key regardless of input length. Verified live: duplicate requests with the same idempotency key return the same notification ID, and the actual Redis key matches the SHA-256 of the literal string used.

**Dedup TTL moved from a hardcoded constant to `config.DedupTTL` (`DEDUP_TTL_HOURS`, operational fix)**
The 24h dedup window was a package-level `const`, unchangeable without a code change and redeploy. Added `DedupTTL time.Duration` to `Config` (default 24h, validated positive in `Validate()`), threaded through `dedup.New(client, ttl)` and `main.go`. Added `DEDUP_TTL_HOURS=24` to `.env.example`. Verified the wiring end-to-end with the default value; didn't separately test a non-default TTL, since the value passes straight through to Redis's own `EXPIRE`.

**Sliding-window-to-token-bucket rewrite rejected, not implemented**
The review flagged the per-request ZSET member as memory-expensive at very high volume (50k req/min) and suggested a token-bucket algorithm instead. Rejected for now: the review itself frames this as a high-scale concern this project is nowhere near (no production traffic), a token bucket is a different algorithm with different semantics (smoothed approximation vs. exact counting) rather than a drop-in swap, and tuning a bucket's refill rate without a real traffic pattern risks getting it wrong. Logged as a deferred Phase 13+ item, gated on an actual observed ZSET memory problem.

**Replayer loop given bounded concurrency instead of `pgx.Batch` (different approach than suggested)**
`replayOnce` processed up to 100 candidates fully sequentially, each doing a rate check, a deliveries lookup, an optional Kafka publish, and two separate SQL status updates — serializing the whole chain per notification on one background goroutine. The review's suggested `pgx.Batch` only covers the two SQL calls and doesn't touch the rate check or Kafka publish, which are the more expensive steps and aren't batchable SQL anyway. Applied the same bounded-goroutine-concurrency pattern already used for `NotificationHandler.Batch` (phase 4): `replayOnce` now runs each `replayOne` in its own goroutine, capped at 16 concurrent via a semaphore. Verified live: rate-limited a tenant down to 3 sends, cleared the Redis rate keys, and confirmed one replayer tick correctly transitioned all 3 held-back notifications to `queued`.

## Phase 6 — Channel Integrations — 2026-06-20

**Provider clients built as plain REST wrappers (Resend, Fast2SMS) and a manual stdlib JWT/OAuth2 flow (FCM), no new dependencies**
Resend and Fast2SMS each need exactly one HTTP call (a JSON POST and a query-param GET respectively), so both were implemented directly against `net/http` rather than pulling in SDKs. FCM's HTTP v1 API only needs a bearer token, not the full Admin SDK, so instead of adding `firebase.google.com/go/v4` (which drags in the broader Google Cloud client stack), the service-account JWT bearer flow (RFC 7523) was implemented directly with `crypto/rsa`, `crypto/x509`, and `crypto/sha256`: sign a JWT assertion with the service account's RSA key, exchange it for an access token at Google's token endpoint, cache the token until near expiry, and call FCM's send endpoint with it. `go.mod` is unchanged — verified via `go build ./...`/`go vet ./...` passing with no `go mod tidy` needed. Verified all three clients with real HTTP-round-trip tests (`httptest.Server`, not mocks), including a full real JWT-sign → token-exchange → FCM-send chain in the FCM test using a throwaway generated RSA key.

**Fast2SMS logical failures detected via response body, not HTTP status**
Fast2SMS returns HTTP 200 even when a send is rejected (bad number, no balance) — the actual outcome is in the JSON body's `return` boolean. `Send` explicitly checks `sr.Return` and surfaces `sr.Message[0]` as the error reason when false, rather than trusting the status code.

**Delivery status recorded once on success in the channel handler; failure recorded once in the DLQ consumer, not on every retry**
The existing Kafka consumer (`handleWithRetry`) retries a failed handler up to 3 times before routing to DLQ, so a naive "update delivery status on every handler failure" would inflate `attempts` and prematurely mark a delivery failed before retries are exhausted. Instead, the email/push/sms handlers only call `UpdateStatus(..., StatusDelivered, "")` on success (which ends the retry loop immediately, so it only ever fires once); the terminal `StatusFailed` is recorded exactly once in `DLQConsumer`, which is only ever reached after all retries are exhausted. `DLQConsumer` was also extended to mark the parent `Notification` as failed, not just write the DLQ row.

**Provider consumers are optional at startup, same convention as Kafka/Redis**
`startConsumers` only constructs and starts the email/push/sms consumers if their corresponding config (`RESEND_API_KEY`, `FAST2SMS_API_KEY`, `FIREBASE_CREDENTIALS_JSON`) is non-empty; otherwise it logs a warning and the app still boots. This mirrors the existing optional-Kafka/optional-Redis pattern and lets local/dev runs work without every provider credential configured. Verified live: server boots cleanly with no Kafka/Redis/provider env vars set (the Kafka-disabled path skips `startConsumers` entirely); the Kafka-enabled provider-optional branches were verified by `go build`/`go vet` passing, since no local Kafka broker was available to drive a live end-to-end run.

**Added `ResendFromEmail` config, required only if `ResendAPIKey` is set**
Resend rejects sends without a verified "from" address, so `RESEND_FROM_EMAIL` was added to `Config` and validated as required whenever `RESEND_API_KEY` is set (and only then), keeping the no-provider-configured path unaffected.

## Phase 7 — WebSocket & In-app — 2026-06-20

**Single-instance delivery model: `ws.Hub` is authoritative, Redis `presence.Tracker` is a secondary record, not implemented**
This project doesn't horizontally scale, so there's no cross-instance fan-out (no Redis pub/sub relaying messages between processes) — a recipient connected to process A is invisible to process B. `ws.Hub.IsOnline`/`SendToUser` (in-memory, per-process) are the only source of truth the in-app consumer checks before deciding "online vs. queue offline." `presence.Tracker` (Redis-backed, 2-minute TTL) still exists and is updated on connect/disconnect, but purely as an observability record for a future multi-instance setup — nothing currently reads it to make a delivery decision. Revisit if/when this project ever runs more than one server process.

**WS handshake authenticated via `apiKey` query param, not the `X-API-Key` header**
Browsers' native `WebSocket` API can't set custom headers on the handshake request, so the existing header-based `middleware.Auth` (used by all `/api/v1` routes) doesn't work for `/ws/connect`. The handler does its own lookup (`apiKeyRepo.GetTenantByKeyHash`) and additionally checks the resolved tenant's ID against the `tenantId` query param, so a valid key for tenant A can't open a socket claiming to be tenant B. Registered directly on the root Echo instance rather than under the `authMW` group, since its auth model is bespoke. Verified live: connecting with a valid key+matching tenantId succeeds (101 upgrade, logged "websocket connected"/"websocket disconnected" on clean close); a bad key and a mismatched tenantId each correctly return 401, and missing params return 400.

**In-app delivery reuses the existing Phase 6 retry/DLQ machinery for the "offline, no durable queue" case, rather than a new failure path**
`NewInAppConsumer`'s handler: if `hub.SendToUser` succeeds, mark `StatusDelivered` (same as email/push/sms on success). If the recipient is offline and `offlinequeue.Queue` is configured (`REDIS_URL` set), push the payload and mark `StatusQueued` via `SetStatus` — not `UpdateStatus`, since queuing isn't a delivery attempt, mirroring how the Phase 5 replayer marks `queued_rate_limited` rows. If offline with no queue configured, the handler returns an error instead of writing any status itself, letting the same 3-attempt-retry-then-DLQ path from Phase 3/6 record the terminal failure — avoiding a second, parallel "mark as failed" code path for what's ultimately the same kind of permanent non-delivery.

**`main.go` Redis setup split in two: Redis-dependent objects built before the Kafka block, `Replayer` construction kept after it**
The in-app consumer's `offlinequeue.Queue` is needed at Kafka-consumer-construction time, but `Replayer` needs the real (possibly-nil) Kafka `*producer.Producer` value and must not be constructed before the Kafka block runs (it takes `prod` by value, so building it early would freeze it at `nil` even when Kafka is actually configured). Split the old single `if cfg.RedisURL != ""` block into "construct redis client / limiter / dedup / presence.Tracker / offlinequeue.Queue" (moved earlier, before Kafka) and "construct + run `Replayer`" (kept in its original relative position, after the Kafka block, now gated on `redisClient != nil`). `ws.Hub` itself has no external dependency and is constructed unconditionally.

**Verified live without Kafka/Redis running locally**
`go build ./...` and `go vet ./...` pass. Ran the server against a local Postgres only (no Kafka, no Redis) to confirm both still degrade gracefully — boots cleanly, logs the existing "REDIS_URL not set"/"KAFKA_BOOTSTRAP_SERVERS not set" warnings, `/health` responds. Created a tenant via the API, then dialed `/ws/connect` with a real `gorilla/websocket` client using its API key: handshake succeeds, logs "websocket connected", clean disconnect logs "websocket disconnected". Did not verify actual message delivery over the socket or the offline-queue flush-on-reconnect path live, since both require either a running Kafka broker (to drive the in-app consumer) or a running Redis (for the queue) — neither was available locally; that logic was verified by code inspection only.

## Phase 8 — Templates — 2026-06-20

**Templates are tenant-scoped CRUD over the existing `notification_templates` table — no migration needed**
The table (`tenant_id, name, channel, subject, body`, `UNIQUE(tenant_id, name)`) and the `notifications.template_id` FK already existed from the Phase 1 schema, and `domain.NotificationTemplate` already documented `{{variable}}` substitution as the intended behavior. Phase 8 was purely application-layer: `TemplateRepository` (same `Executor`-backed, tenant-filtered pattern as every other repo), `TemplateHandler` (same shape as `tenant.go`/`notification.go` — `Validate()` DTOs, `TenantFromContext` for auth), and routes registered under the existing authenticated `/api/v1` group.

**Duplicate template name surfaced as 409, not 500 — new `ErrAlreadyExists`/`isUniqueViolation` convention**
No prior repository needed to distinguish a unique-constraint violation from any other DB error, but `UNIQUE(tenant_id, name)` makes a duplicate template name a real, expected user mistake worth a distinct response. Added `isUniqueViolation(err) bool` (checks `pgconn.PgError.Code == "23505"`) and `ErrAlreadyExists` to `errors.go`; `TemplateRepository.Create`/`Update` map the constraint violation to it, and the handler maps that to HTTP 409.

**Rendering happens once, at send time, before persisting/publishing — not in each channel consumer**
The milestone wording ("wire template rendering into all channel consumers") reads as if rendering should happen consumer-side, but every consumer already receives a fully-formed `Subject`/`Body` from the notification row/Kafka message — duplicating template lookup and `{{variable}}` substitution into four separate consumers would mean four copies of the same logic, plus each consumer needing its own `TemplateRepository` wiring, for no behavioral benefit (the rendered text is the same regardless of which channel delivers it). Instead, `NotificationHandler.resolveTemplate` runs once in `Send`/`Batch`, before `processSend`: it loads the template (tenant-scoped, 404 if missing), checks `template.Channel` matches the request's channel (422 on mismatch), and renders `Subject`/`Body` via `internal/template.Render` against the caller's `variables` map. The rendered text — not the template reference — is what gets persisted and published, so every consumer continues to work unmodified; `template_id` is still stored on the notification row for traceability. This satisfies the spirit of the milestone item (rendering reaches every channel) without consumer-side duplication.

**`internal/template.Render` does literal placeholder substitution via regex, missing variables left as-is**
`{{key}}` (with optional internal whitespace) is replaced by `vars[key]`; a placeholder with no matching key is left untouched rather than blanked out, so a missing variable is visible in the rendered output (easier to debug than a silently empty substitution) instead of disappearing. No nested/conditional templating — not needed for this project's use case.

**Verified live against local Postgres**
`go build ./...` / `go vet ./...` pass. Created a tenant, enabled the email channel, created a template with `{{first_name}}`/`{{code}}` placeholders, and sent a notification with `template_id` + `variables` — confirmed the persisted notification has the correct `template_id`, and `subject`/`body` rendered to the literal substituted text. Verified a duplicate template name returns 409, update/delete work and tenant-scope the lookup correctly (404 after delete), and that sending against a disabled channel is still rejected before template resolution runs.

## Phase 9 — Scheduler & Cleanup — 2026-06-20

**Scheduling reuses `createAndQueue`'s persist transaction with `publish=false`, rather than a separate code path**
`POST /notifications/schedule` builds the same `postgres.CreateNotificationParams` shape Send/Batch use (including optional `template_id`/`variables` via the existing `resolveTemplate`), sets `Status: domain.StatusPending` and `ScheduledAt: &scheduledAt`, and calls `h.createAndQueue(ctx, params, false)` — the existing helper already branches on its `publish` bool to skip the Kafka step, so no new persist logic was needed. After the transaction commits, a `scheduled_notifications` row is created separately pointing at the new notification's ID. This keeps Send/Schedule/Batch sharing one transactional persist path instead of three.

**`ScheduledAt` added to `CreateNotificationParams`/`Create`'s INSERT — the column and `domain.Notification` field already existed, only the write side was missing**
`notifications.scheduled_at` was already selected by every query via `scanNotification`, and `domain.Notification.ScheduledAt *time.Time` already existed from Phase 1 — `Create`'s INSERT just never set it because nothing needed to until now. Added as a 15th bound param, nil for immediate Send/Batch sends.

**New `scheduler` package mirrors the existing `ratelimit.Replayer` background-worker pattern exactly**
`Scheduler.Run(ctx, interval)` ticks every 30s (`scheduleInterval` in `main.go`, same convention as `replayInterval`), calling `ScheduledNotificationRepository.GetDueUnfired` (backed by the existing partial index `WHERE fired = FALSE`) and firing up to 16 due rows concurrently per tick. Wired into the same `bgCtx`/`bgWg` background-worker group in `main.go`, drained on shutdown identically to the replayer and Kafka consumers. Unlike the replayer, the scheduler isn't gated on `redisClient != nil` — scheduling has no Redis dependency, so it always starts.

**A scheduled row is left unfired (not silently dropped) when Kafka isn't configured**
`createAndQueue`'s immediate-send path logs a warning and treats "no producer" as success, because there's no later retry mechanism for that case — the notification has nowhere else to be redelivered from. The scheduler is different: it polls on every tick, so if `fireOne` returns without calling `MarkFired` when `producer == nil`, the same row is simply retried on the next tick once Kafka is wired up, with nothing lost. Chose retry-by-omission over a separate "kafka unavailable" status to avoid adding a new `domain.Status` value for what's already self-healing.

**Daily cleanup is a plain ticker goroutine in `main.go`, not a separate package**
`NotificationRepository.DeleteExpired(ctx, now)` is one `DELETE FROM notifications WHERE expires_at < $1` statement (deliveries cascade-delete via the existing FK) — wrapping it in its own package would be overhead for a single query. The 24h-interval ticker loop lives directly in `main.go` inside the same `bgWg`/`bgCtx` group as everything else, logging a count only when rows were actually deleted.

**Verified live against local Postgres (no Kafka running)**
`go build ./...` / `go vet ./...` pass. Enabled the email channel, scheduled a notification 5s in the future — confirmed the response and the persisted row both show `status: pending` with `scheduled_at` set, and no Kafka publish occurs at create time. Confirmed `scheduled_at` validation rejects past timestamps and missing values (422). Waited through two scheduler ticks (30s apart): each logged `"scheduler: kafka not configured — scheduled notification due but not published"` and left the `scheduled_notifications` row `fired = false` and the notification `status = pending` — confirming the no-Kafka retry-by-omission behavior rather than a false "fired" state. Verified the cleanup query's `WHERE expires_at < NOW()` logic directly against an inserted already-expired row (deleted correctly) — the 24h ticker itself wasn't run end-to-end live since that would require waiting a day, but the query it executes is the same one verified directly.

## Phase 10 — Analytics — 2026-06-20

**New `AnalyticsRepository` reads `notifications`/`dlq_messages` directly rather than going through `NotificationRepository`/`DLQRepository`**
These are aggregate queries (`COUNT(*) FILTER (...)`, `GROUP BY channel`, a `generate_series` day join) — not row-shaped CRUD, so they don't fit the existing repos' single-row/paginated-list method shapes. A separate repo keeps `NotificationRepository` focused on persisting/reading individual notifications.

**`GET /api/v1/tenants/:id/analytics` is unauthenticated, alongside the other `/tenants/:id/*` routes — not under the `X-API-Key`-gated `/api/v1` group**
Mirrors the existing tenant-route convention (`Get`, `UpdateChannels`, `UpdateRateLimits` all take `:id` as a path param with no auth, since tenant management is super-admin/open for portfolio purposes — see Phase 2 decision). Analytics is consumed by the Flutter dashboard per-tenant, the same audience as the other tenant routes, so it follows the same pattern rather than introducing a second auth model for one endpoint.

**`from`/`to` default to the last 30 days, both optional, both RFC3339 — same validation convention as `historyQueryParams`**
Reuses the exact `time.Parse(time.RFC3339, ...)` + 400-on-parse-failure pattern already used by `/notifications/history`. Defaulting `to` to `time.Now()` and `from` to `to.AddDate(0, 0, -30)` means the endpoint is useful with no query params at all, which matters for a dashboard's default "last 30 days" view.

**DLQ trend fills every day in range with `generate_series`, including zero-count days, rather than only returning days that have DLQ rows**
A chart with gaps for zero-DLQ days would be misleading (looks like missing data, not "zero incidents"). Used a `LEFT JOIN` against a `generate_series(date_trunc('day', from), date_trunc('day', to), interval '1 day')` series — first version aliased the series column as `day` and hit `ERROR: column reference "day" is ambiguous` against the same-named subquery column; fixed by aliasing the series as `g(day)` and the subquery's column as `c.count`/joining on `c.day = g.day` explicitly.

**Channel breakdown only includes channels with at least one notification in range (`GROUP BY channel` with no zero-fill)**
Unlike the DLQ trend, an empty channel isn't an interesting "zero" data point for a dashboard pie/bar chart — channels a tenant never used shouldn't appear at all, so no need for a fixed `domain.Channel` enumeration join like the day-series join above.

**Verified live against local Postgres**: created a tenant, enabled email + sms, sent 4 notifications, then directly updated 2 to `delivered` and 1 to `failed` via SQL (no Kafka consumers running locally to drive real status transitions) and inserted 2 `dlq_messages` rows (one "today", one backdated one day) for the failed notification. `GET /tenants/:id/analytics` returned `total_sent: 4, total_delivered: 2, total_failed: 1`, a channel breakdown of `email: 3 sent/2 delivered (0.667 rate)` and `sms: 1 sent/1 failed (0 rate)`, and a 30-point DLQ trend with `count: 1` on each of the two seeded days and `0` everywhere else. Also verified 404 for an unknown tenant ID and 400 for an unparseable `from` query param.

## Phase 11 — Observability — 2026-06-20

**New `internal/metrics` package instrumented at shared chokepoints, not per-route**
Five Prometheus collectors (`NotificationsTotal`, `RetriesTotal`, `DLQTotal`, `ChannelSendDuration`, `HTTPRequestDuration`) registered via `promauto` and scraped at `/metrics` (`promhttp.Handler()`, unauthenticated like `/health`). Rather than instrumenting every HTTP handler individually, counters are incremented at the few places every send/batch/schedule path and every channel consumer already funnels through: `createAndQueue` (queued count, by channel+status), `Consumer.handleWithRetry`'s retry loop (`RetriesTotal`) and its DLQ-publish branch (`DLQTotal`), the per-channel consumers (`ChannelSendDuration` around the provider call, `NotificationsTotal` on delivered), and `dlq.go` (`NotificationsTotal` on terminal failed). `HTTPRequestDuration` is the one route-level metric, applied via `metrics.EchoMiddleware()` and labeled by `c.Path()` (the route pattern, e.g. `/api/v1/tenants/:id`) rather than the raw request path, to avoid unbounded label cardinality from path params like tenant/notification IDs.

**OpenTelemetry tracing exports to a local `traces.jsonl` file via `stdouttrace`, not a collector backend**
No OTel collector/Jaeger/Tempo exists in this deployment, so `internal/tracing.Init` wires a `stdouttrace` exporter writing JSON-lines to a local file (`os.OpenFile` with `O_CREATE|O_WRONLY|O_APPEND`) instead of shipping spans remotely — explicitly a placeholder for swapping in an OTLP exporter once Phase 13's docker-compose adds a real backend. A single shared tracer (`tracing.Tracer()`, instrumentation scope `"notifyx"`) is used everywhere rather than per-package tracer names, since this is one service and spans are already distinguished by name/attributes. The span chain follows the milestone's required shape: `notification.create_and_queue` (API handler) → `kafka.publish` (producer) → `kafka.consume` (per retry attempt, in the consumer's retry loop). The channel-provider HTTP call itself wasn't given a separate span — it happens inside `kafka.consume`'s context, and a fourth span tier was judged unnecessary granularity for this scope.

**Resource construction avoids `resource.Merge`/`resource.Default()` — they conflict on schema URL (bug fix)**
First version built the OTel resource via `resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, ...))`, which crashed at startup with `conflicting Schema URL: .../1.41.0 and .../1.26.0` — the SDK's built-in `resource.Default()` (otel SDK v1.44.0) uses a newer semconv schema than the explicit `semconv/v1.26.0` import used for `ServiceName`/`DeploymentEnvironment`. Fixed by constructing the resource directly via `resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("notifyx"), semconv.DeploymentEnvironment(env))`, skipping the merge entirely.

**Grafana/Prometheus config added under `docker/` now, as a forward reference for Phase 13**
`docker/prometheus.yml` (scrapes `host.docker.internal:8080`, target to be changed to the compose service name once docker-compose.yml exists), `docker/grafana/provisioning/datasources/prometheus.yml` (default Prometheus datasource), `docker/grafana/provisioning/dashboards/dashboards.yml` (file-based dashboard provider, `path: /etc/grafana/dashboards`), and `docker/grafana/dashboards/notifyx.json` (a 5-panel dashboard: notifications/sec by channel+status, DLQ/sec by channel, retries/sec by channel, channel send latency p50/p95/p99, HTTP latency p95 by route) — none of this is wired into a running container yet since docker-compose.yml is a Phase 13 deliverable, but the configs are ready to be volume-mounted when it's built.

**Verified live against local Postgres (no Kafka running)**: `go build ./...` / `go vet ./...` pass. Ran the server, created a tenant, enabled the email channel via `PUT /tenants/:id/channels` with the correct `{"channels":[{"channel":"email","enabled":true}]}` body shape, and sent a notification. `/metrics` showed `notifyx_notifications_total{channel="email",status="pending"} 1` and multiple `notifyx_http_request_duration_seconds_count{...}` rows, including a `status="403"` row from an earlier attempt that used the wrong channel-update body shape and got correctly rejected. `traces.jsonl` contained a valid `notification.create_and_queue` span. Kafka-dependent metrics (`RetriesTotal`, `DLQTotal`, `ChannelSendDuration`) and spans (`kafka.publish`, `kafka.consume`) weren't exercised locally since `KAFKA_BOOTSTRAP_SERVERS` is unset — consistent with every prior phase's local-verification limitation around the optional-Kafka convention.
