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
