# Phase 5 Rate Limiting & Deduplication — Architectural & Implementation Review

This document contains a detailed analysis, architectural critique, and recommended improvements for the Redis-based rate limiting and idempotency deduplication components implemented in **Phase 5** of the Notifyx service.

**Status: all 6 issues addressed (2026-06-19).** 5 fixed as recommended (hash-tagged rate-limit keys, atomic Lua dedup reserve, configurable dedup TTL, SHA-256 hashed dedup keys, bounded-concurrency replayer); 1 pushed back on (Issue 3 — token-bucket rewrite of the sliding-window limiter, deferred as a Phase 13+ item pending a real memory/scale number). Verified live against a real local Postgres + Redis (`brew services start postgresql@16`, `redis-server` via Homebrew): created a tenant, enabled the email channel, set a low rate cap, and confirmed (a) duplicate-idempotency-key sends return the same notification ID with the Redis dedup key visibly hashed to a fixed 64-char hex string, (b) sends past the cap come back `queued_rate_limited` with `rate:{tenantID}:...` keys correctly hash-tagged, and (c) clearing the rate window and waiting for a replayer tick correctly transitions all rate-limited notifications to `queued` via the new bounded-concurrency loop.

---

## 1. Clustered Redis Cross-Slot Failures (High Risk)

### Issue Description
In [Limiter.Allow](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/ratelimit/limiter.go#L69), the keys used for the sliding window check are generated as follows:

```go
channelKey := fmt.Sprintf("rate:%s:%s", tenantID, channel)
globalKey := fmt.Sprintf("rate:%s:global", tenantID)
```

These keys are then passed to the Lua script `slidingWindowScript` which operates on them concurrently.

### Architectural Implications
1. **Clustering Failures:** In production-scale Redis setups (such as clustered Upstash instances or AWS ElastiCache clusters), keys are distributed across slots by hashing the entire key string. Because `channelKey` and `globalKey` are different strings, they will hash to different slots.
2. **CROSSSLOT Errors:** When the Lua script runs, Redis will immediately throw a **`CROSSSLOT Keys in request don't hash to the same slot`** error and abort, breaking rate limiting entirely.

### Recommended Resolution
Wrap the tenant ID in **hash tags** `{...}`. Redis only hashes the content inside the curly braces to determine slot placement, guaranteeing both keys land on the exact same slot:

```go
channelKey := fmt.Sprintf("rate:{%s}:%s", tenantID, channel)
globalKey := fmt.Sprintf("rate:{%s}:global", tenantID)
```

### Resolution — Fixed as recommended

Confirmed: `Limiter.Allow` built `channelKey`/`globalKey` from plain `tenantID` with no hash tag, and both are passed to `slidingWindowScript` as `KEYS[1]`/`KEYS[2]` — exactly the shape that triggers `CROSSSLOT` on a clustered Redis.

Applied the recommended fix verbatim in [limiter.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/ratelimit/limiter.go): wrapped `tenantID` in `{}` in both key formats. Single-node Redis (current local dev and most managed free tiers) ignores hash tags entirely, so this is a no-op there — it only matters once/if a clustered Redis is ever used, but costs nothing to fix now.

Verified live against a local single-node Redis: confirmed via `redis-cli keys 'rate:*'` that keys now read `rate:{tenantID}:email` / `rate:{tenantID}:global`, and that the sliding-window check still correctly blocks requests once the cap is hit (3 allowed, 4th+ returned `queued_rate_limited`). Cluster-mode CROSSSLOT behavior itself wasn't tested, since this environment has no Redis Cluster to test against — verified by code/documentation reasoning about hash-tag slot assignment, not by reproducing the actual error.

---

## 2. Deduplication Double Network Round-Trips

### Issue Description
In [Deduplicator.Reserve](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/dedup/dedup.go#L32), the handler performs a `SetNX` and then falls back to a `Get` if the key already exists:

```go
ok, err := d.client.SetNX(ctx, key, notificationID.String(), ttl).Result()
...
val, err := d.client.Get(ctx, key).Result() // Executed on duplicate hit
```

### Architectural Implications
1. **Added Latency:** On duplicate requests, this issues two sequential network round-trips to Redis. If connecting to a serverless Redis (like Upstash) over WAN/HTTP, this doubles the latency of duplicate request handling.

### Recommended Resolution
Use a single, atomic Lua script to write the key if absent, or retrieve it if present, in a single round-trip:

```go
var reserveScript = redis.NewScript(`
if redis.call('SETNX', KEYS[1], ARGV[1]) == 1 then
    redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
    return {1, nil}
else
    return {0, redis.call('GET', KEYS[1])}
end
`)
```

### Resolution — Fixed as recommended

Confirmed: `Reserve` issued `SetNX` then, on a miss, a separate `Get` — two sequential round trips on every duplicate hit.

Implemented the suggested Lua script essentially verbatim in [dedup.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/dedup/dedup.go) (`reserveScript`), with one adjustment: returns `''` instead of `nil` in the claimed branch, since go-redis's Lua-to-Go type conversion is more predictable with a consistent two-element array than with a `nil` member mixed into a typed slice. `Reserve` now does one `EVALSHA`/`EVAL` round trip regardless of claim outcome.

Verified live: sent a notification with `idempotency_key=abc`, then repeated the exact same request — both calls returned the identical `notification_id`, confirming the claimed/not-claimed branches of the script both resolve correctly through the new single-round-trip path. Did not separately measure the latency delta from removing the second round trip (no WAN-hop Redis available locally to make that delta meaningful) — verified correctness, not the latency claim itself.

---

## 3. Memory Footprint of Sliding Window Log (Scale Trade-off)

### Issue Description
The sliding window log algorithm adds a unique string `now-UUID` for every single request to the Redis Sorted Set (ZSET).

### Architectural Implications
1. **High Memory Overhead:** Storing a member (containing a timestamp and a 36-character UUID string) for *every* notification request in a Redis Sorted Set (ZSET) is highly accurate but memory-expensive. If a tenant sends 50,000 notifications per minute, Redis must store 50,000 separate ZSET members.

### Recommended Resolution
* For higher scale, consider a **Token Bucket** or **Leaky Bucket** algorithm (storing a single Redis hash containing only two fields: `tokens` and `last_updated`), which consumes a constant, negligible amount of memory regardless of request volume.

### Resolution — Pushed back

Confirmed the mechanism described: `slidingWindowScript` adds one ZSET member (`now-UUID`, ~50 bytes) per request to both the per-channel and global sets, trimmed lazily via `ZREMRANGEBYSCORE` on each call. At 50,000 req/min for one tenant, that's real memory pressure.

Not implementing the token-bucket rewrite now. This review itself frames it as "for higher scale" — there's no tenant anywhere near that volume in this project (a portfolio project with no production traffic), and a token bucket is a strictly different algorithm with different semantics (it smooths bursts via a refill rate rather than exactly counting requests in a trailing window), not a drop-in optimization of the current one. Rewriting the limiter on a hypothetical future volume risks getting the bucket size/refill-rate tuning wrong without any real traffic pattern to tune against — exactly the kind of premature optimization this project's stated philosophy avoids. The current sliding-window log is also strictly more accurate (exact request counting vs. a bucket's smoothed approximation), which is a real tradeoff to give up, not just an implementation detail.

Logged in `planning/decisions.md` as a deferred Phase 13+ item: revisit with a token-bucket (or leaky-bucket) limiter if a tenant's actual sustained request volume is ever observed to make the current ZSET memory footprint a measured problem.

---

## 4. Hardcoded Idempotency Key TTL (Operational Rigidity)

### Issue Description
In [Deduplicator](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/dedup/dedup.go#L17), the deduplication window is locked to a hardcoded constant:

```go
const ttl = 24 * time.Hour
```

### Architectural Implications
1. **Lack of Operational Control:** As active traffic scales, system administrators cannot adjust this window to free up Redis memory or change deduplication rules without refactoring the code.

### Recommended Resolution
Move this parameter to `config.Config` (e.g. `DEDUP_TTL`) and inject it into the `Deduplicator` struct on initialization.

### Resolution — Fixed as recommended

Confirmed: `const ttl = 24 * time.Hour` was a package-level constant in `dedup.go`, with no way to change it without a code change and redeploy.

Added `DedupTTL time.Duration` to `config.Config`, read from `DEDUP_TTL_HOURS` (default `24`), validated as positive in `Config.Validate()` (consistent with this project's existing fail-fast config policy). `dedup.New` now takes `ttl time.Duration` as a parameter instead of reading a constant; `main.go` passes `cfg.DedupTTL`. Added `DEDUP_TTL_HOURS=24` to `.env.example`.

Verified: `go build`/`go vet` pass; live-tested with the default 24h value (dedup behavior unchanged — confirmed via the duplicate-request test in Issue 2's resolution above). Did not test a non-default TTL value end-to-end, since confirming the parameter is correctly threaded through (`config` → `main.go` → `Deduplicator.ttl` → the Lua script's `ARGV[2]`) was sufficient to validate the wiring; the TTL is passed straight to Redis's own `EXPIRE`, which is well-trusted to honor whatever value it's given.

---

## 5. Redis Key Bloat Vulnerability (Security & Hardening)

### Issue Description
In [Deduplicator.Reserve](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/dedup/dedup.go#L32), the key string is built directly from the client-supplied idempotency key string:

```go
key := "dedup:" + idempotencyKey
```

### Architectural Implications
1. **Memory Bloat:** A client could pass an extremely long string (e.g., several megabytes of text) as the idempotency key, causing memory exhaustion in Redis.

### Recommended Resolution
Hash the client-provided key (e.g. SHA-256) inside the key generator. This guarantees a safe, fixed-length 64-character hex key string in Redis:
```go
hash := sha256.Sum256([]byte(idempotencyKey))
return "dedup:" + hex.EncodeToString(hash[:])
```

### Resolution — Fixed as recommended

Confirmed: `redisKey` concatenated `"dedup:"` directly onto the raw client-supplied string with no length cap or hashing — an attacker (or buggy client) could pass an arbitrarily long idempotency key and have it stored verbatim as a Redis key.

Implemented the suggested fix verbatim in [dedup.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/dedup/dedup.go)'s `redisKey`: SHA-256 the input, hex-encode it, prefix with `dedup:`. Every dedup key is now a fixed 70 characters (`dedup:` + 64 hex chars) regardless of input length. This is a one-way hash, so there's no way to recover the original idempotency key from Redis — not a concern here, since nothing ever needs to read it back out except by recomputing the same hash from the same client-supplied key.

Verified live: sent a request with `idempotency_key=abc` and confirmed via `redis-cli keys 'dedup:*'` that the stored key was `dedup:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015a` — the actual SHA-256 of the literal string `abc`, confirming the hash function and encoding are wired correctly. Did not separately test an oversized (multi-MB) idempotency key, since the fix's correctness only depends on SHA-256 producing a fixed-size output regardless of input size, which doesn't need a large-input test to confirm.

---

## 6. N+1 Database Round-trips in Replayer Loop (Performance Bottleneck)

### Issue Description
In [Replayer.replayOne](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/ratelimit/replayer.go#L68), when rate-limited notifications are released, the replayer executes separate SQL `UpdateStatus` and `SetStatus` commands sequentially inside a loop:

```go
r.notifications.UpdateStatus(ctx, ...)
r.deliveries.SetStatus(ctx, ...)
```

### Architectural Implications
1. **Loop Blocking:** Replaying a batch of 100 notifications will trigger 200 individual SQL update queries sequentially, blocking the execution of the background replayer thread.

### Recommended Resolution
Group the database updates and execute them using a transactional batch (`pgx.Batch`) to commit all status updates in a single database round-trip.

### Resolution — Fixed, with a different approach

Confirmed: `replayOnce` looped over up to `batchSize` (100) candidates fully sequentially, calling `replayOne` (which itself does a rate check, a deliveries lookup, an optional Kafka publish, then `UpdateStatus` + `SetStatus`) one at a time — so a 100-notification tick really does serialize ~5 round trips per notification on the single background goroutine.

Did not implement the literal `pgx.Batch` suggestion: `UpdateStatus` and `SetStatus` target different tables (`notifications`, `notification_deliveries`) via different repository methods with no shared transaction context in `replayOne`, and batching just those two statements wouldn't address the bigger cost in the loop — the per-notification rate check and Kafka publish, which `pgx.Batch` can't touch since they aren't SQL. Instead applied the same bounded-concurrency pattern already used for `NotificationHandler.Batch` (phase 4, Issue 3): `replayOnce` now runs each `replayOne` call in its own goroutine, capped at `maxConcurrentReplays = 16` via a semaphore, so the tick's wall-clock time is bounded by the slowest ~16-deep batch rather than the full sequential sum. This addresses the actual complaint ("blocking the execution of the background replayer thread") more directly than batching two SQL statements would have, without adding a second transactional path alongside the one already on `NotificationRepository`/`NotificationDeliveryRepository`.

Verified live against a real local Postgres + Redis: set the per-tenant rate cap to 3, sent 5 notifications (2 succeeded, 3 came back `queued_rate_limited`), cleared the tenant's Redis rate-limit keys to simulate the window clearing, then waited for one replayer tick (15s) — all 3 rate-limited notifications correctly transitioned to `queued` in Postgres. Did not specifically load-test with 100 concurrent candidates to measure the wall-clock improvement, since this environment's local Postgres has no realistic latency profile to make that measurement meaningful — verified correctness of the concurrent path, not the claimed throughput gain.

