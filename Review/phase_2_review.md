# Phase 2 Tenant & Auth — Architectural & Implementation Review

This document contains a detailed analysis, architectural critique, and recommended improvements for the tenant management and authentication components implemented in **Phase 2** of the Notifyx service.

**Status: all 6 issues addressed (2026-06-19).** 5 fixed as recommended (transactions, key rotation/revocation, unsafe casting, `rows.Err()`, CORS); 1 (Redis auth-cache) acknowledged as valid but deferred — see its Resolution for why. Each fix was verified by building, then running the server against a real local Postgres (Docker `aef-postgres`, fresh `notifyx_test` database, migrations applied at boot) and exercising the actual HTTP endpoints with curl, not just `go build`.

---

## 1. Lack of Database Transactions (Atomicity Violations)

### Issue Description
The handlers for tenant creation and bulk update settings execute multiple distinct queries on the database connection pool without transaction wrapping.

#### Case A: Tenant and API Key Generation
In [tenant.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/tenant.go#L106), tenant creation and key storage are separate database operations:
```go
tenant, err := h.svc.Tenants.Create(c.Request().Context(), req.Name)
if err != nil { ... }

rawKey, keyHash, err := generateAPIKey()
if err != nil { ... }

if _, err := h.svc.APIKeys.Create(c.Request().Context(), tenant.ID, keyHash); err != nil { ... }
```

#### Case B: Settings Batch Updates
In `UpdateChannels` and `UpdateRateLimits` handlers, settings updates are processed one-by-one inside a loop:
```go
for _, rl := range req.RateLimits {
    if err := h.svc.RateLimits.Upsert(ctx, id, domain.Channel(rl.Channel), rl.MaxPerMin, rl.GlobalCap); err != nil {
        return errResponse(c, http.StatusInternalServerError, "failed to update rate limits")
    }
}
```

### Architectural Implications
1. **Orphaned Records:** If step 2 of tenant creation fails, the tenant is successfully persisted in the database but lacks any authentication keys, creating useless orphaned rows.
2. **Partial Updates (Atomicity Violation):** If the third channel update fails in a bulk settings payload, the first two changes remain committed in the database, while the client receives a generic `500 Internal Server Error`. The API lacks all-or-nothing atomicity.
3. **Database Overhead:** Issuing independent updates in a loop causes multiple roundtrips to PostgreSQL, increasing query latency and pool starvation risk.

### Recommended Resolution
* Wrap tenant creation and key insertion in a single PostgreSQL transaction (`pgx.Tx`).
* Perform settings updates inside a single database transaction, or construct a single multi-row batch insert/update statement to ensure full rollback capability on failure.

### Resolution — Fixed as recommended

Valid issue, confirmed against the real code (the snippets above matched [tenant.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/tenant.go) at the time of review, modulo `Upsert`'s exact signature — `RateLimits.Upsert` never took a `GlobalCap` parameter; the global cap has lived on `tenants.global_rate_cap` since the Phase 1 fix, not per-channel-row, so that detail in the review's Case B snippet was already stale).

Added a small `Executor` interface (`Exec`/`Query`/`QueryRow`) in [db.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/db.go) that both `*pgxpool.Pool` and `pgx.Tx` satisfy, plus a `WithTx(ctx, pool, fn)` helper that begins a transaction, runs `fn`, and commits or rolls back. `TenantRepository`, `APIKeyRepository`, `TenantChannelRepository`, and `TenantRateLimitRepository` now take `Executor` instead of `*pgxpool.Pool`, so the exact same repository code runs against the pool (normal path) or a `pgx.Tx` (when a handler needs several writes to commit together) — no query duplication.

`tenant.go`'s `Create` now wraps tenant creation + first API key insert in one `WithTx`; `UpdateChannels` wraps its per-channel upsert loop; `UpdateRateLimits` wraps the global-cap update + per-channel upsert loop. `TenantService` gained a `Pool *pgxpool.Pool` field so handlers can open transactions.

Verified live: ran the server against a real local Postgres, called `POST /api/v1/tenants` and confirmed a tenant + key are created together; called `PUT /:id/channels` and `PUT /:id/rate-limits` and confirmed multi-item updates commit correctly.

---

## 2. High-Load Database Bottleneck: Authentication DB Queries

### Issue Description
The authentication middleware [auth.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/middleware/auth.go#L15) queries PostgreSQL on **every incoming authenticated request**:

```go
keyHash := handlers.HashAPIKey(key)
tenant, err := apiKeyRepo.GetTenantByKeyHash(c.Request().Context(), keyHash)
```

### Architectural Implications
1. **Postgres Connection Starvation:** In a high-throughput notification ingestion API, checking the database for every single incoming request to `POST /api/v1/notifications/send` will clog the database connection pool, drastically lowering maximum throughput.
2. **High Latency:** Every HTTP API request is blocked on a Postgres B-tree index lookup, which is significantly slower than in-memory or Redis key-value cache lookups.

### Recommended Resolution
* Since Redis is already initialized in Phase 5 for rate-limiting, use it to cache the hashed API key -> `Tenant` mapping with a short Time-To-Live (TTL) (e.g. 5–15 minutes).
* Cache entry eviction should occur when a tenant is deleted or their settings are modified.

### Resolution — Acknowledged as valid, deferred

The architectural point is correct: every authenticated request does hit Postgres via `GetTenantByKeyHash`, and that won't scale to high notification-send throughput. Confirmed by reading [auth.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/middleware/auth.go) — no caching layer exists today.

Not implementing this now, for the same reason Phase 1 deferred similar scale-driven work: Notifyx is a portfolio project currently running on free-tier Postgres + Redis with no real production traffic, so a request-path cache adds real complexity (TTL tuning, and — the harder part — correct invalidation on tenant delete *and* key revocation, both of which this same review session just added as new endpoints in Issue 3) for a problem that doesn't exist yet at this load. Building it now would be solving a load problem with no load to measure it against.

Tracked as a checklist item for whenever the project needs to demonstrate handling real throughput (the same Phase 13 polish bucket Phase 1's deferred item landed in): cache `key_hash → tenant` in Redis with a 5–15 min TTL in `auth.go`, and invalidate on `TenantHandler.Delete`, `DeleteKey`, and tenant `Update`. Logged in `planning/decisions.md`.

---

## 3. No API Key Rotation or Revocation Support

### Issue Description
The database schema correctly models multiple API keys per tenant (`api_keys` table uses a unique index and allows multiple keys per `tenant_id`). However, the API does not expose any routes to manage key lifecycle updates.

### Architectural Implications
1. **Security Vulnerability:** If a tenant compromises their API key, there is no self-serve mechanism to rotate or revoke the key.
2. **Destructive Operations:** The only way to revoke a compromised key through the system API is to delete the entire tenant entity, which recursively deletes all their notification templates, histories, and analytics via cascade delete.

### Recommended Resolution
Introduce key rotation and revocation endpoints in [routes.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/routes/routes.go):
* `POST /api/v1/tenants/:id/keys` — Generates and appends a new API key for the tenant (useful for zero-downtime key rotation).
* `DELETE /api/v1/tenants/:id/keys/:key_id` — Revokes and deletes a specific API key hash.

### Resolution — Fixed as recommended

Valid issue — confirmed `routes.go` had no key-management routes, even though `APIKeyRepository` already had unused `Create`/`Delete`/`GetByTenantID` methods sitting idle (built in an earlier phase but never wired to an endpoint).

Added `TenantHandler.CreateKey` (`POST /:id/keys`), `ListKeys` (`GET /:id/keys`), and `DeleteKey` (`DELETE /:id/keys/:key_id`) in [tenant.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/tenant.go), wired into [routes.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/routes/routes.go). `DeleteKey` doesn't verify the key belongs to `:id` first — `api_keys.id` is globally unique and these are already unauthenticated super-admin routes, so the `:id` segment is purely a RESTful URL shape, not an authorization boundary.

Verified live end-to-end: created a tenant (1 key), rotated in a second key via `POST /:id/keys` (confirmed 2 keys via `GET /:id/keys`), revoked the new key via `DELETE /:id/keys/:key_id` (got `204`), and confirmed exactly the original key remained.

---

## 4. Unsafe Interface Casting (Nil Pointer Panic Risk)

### Issue Description
The context retrieval helper [common.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/common.go#L23) uses an unchecked type assertion:

```go
func TenantFromContext(c echo.Context) *domain.Tenant {
	return c.Get("tenant").(*domain.Tenant)
}
```

### Architectural Implications
1. **Thread Panics:** If a developer accidentally registers a route requiring the tenant object without attaching the authentication middleware first, `c.Get("tenant")` will return `nil`. Evaluating `nil.(*domain.Tenant)` throws a runtime panic.
2. **Operational Noise:** While Echo's recover middleware captures this panic and returns a 500 error, crashing request threads pollute logs and crash analytics with unnecessary panic traces.

### Recommended Resolution
Modify the helper to verify type assertions safely:
```go
func TenantFromContext(c echo.Context) (*domain.Tenant, bool) {
	val := c.Get("tenant")
	tenant, ok := val.(*domain.Tenant)
	return tenant, ok
}
```
Or, inside the helper, return a clean error if type assertion fails rather than allowing a thread panic.

### Resolution — Fixed, with a different approach

The unsafe type assertion was real and confirmed in `common.go`. But the review's suggested fix (returning `(*domain.Tenant, bool)`) only moves the panic, not removes it: every caller in `notification.go` does `tenant := TenantFromContext(c)` and then immediately dereferences `tenant.ID` — if a misconfigured route ever omits the auth middleware, swapping the type assertion for a safe one just trades a type-assertion panic for a nil-pointer-dereference panic two lines later. The actual fix has to stop the nil from reaching a dereference at all.

Made `TenantFromContext` do a safe assertion (`tenant, _ := c.Get("tenant").(*domain.Tenant)`, returns nil instead of panicking) and added `RequireTenant(c) (*domain.Tenant, error)` in [common.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/common.go), which calls `TenantFromContext` and writes a clean 500 if it's nil. Updated all 4 call sites in [notification.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/notification.go) (`Send`, `Batch`, `History`, `Get`) from `tenant := TenantFromContext(c)` to `tenant, err := RequireTenant(c); if err != nil { return err }`.

Verified live: called `/notifications/send`, `/history`, and `/notifications/:id` with a valid key (succeeded) and without one (401 from the auth middleware itself, as expected — confirming the route is never reached without a tenant in context on the happy path either way).

---

## 5. Repository Iteration Bug: Unchecked `rows.Err()` (Data Integrity Risk)

### Issue Description
Across all repository read methods querying multiple rows and executing `for rows.Next()` iteration loops, the code fails to verify the final execution state of the query driver.

This issue is present in:
* [TenantRepository.GetAll](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/tenant_repo.go#L49)
* [APIKeyRepository.GetByTenantID](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/api_key_repo.go#L55)
* [TenantChannelRepository.GetByTenantID](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/tenant_channel_repo.go#L35)
* [TenantRateLimitRepository.GetByTenantID](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/tenant_rate_limit_repo.go#L35)

```go
var tenants []*domain.Tenant
for rows.Next() {
    // Scans database rows
}
// <--- Missing rows.Err() check here
return tenants, nil
```

### Architectural Implications
1. **Silent Failures:** If a database node partition, connection pool exhaustion, or database socket crash occurs *mid-way* through scanning the result set, `rows.Next()` returns `false` and exits the loop early.
2. **Data Inconsistency:** The caller receives a truncated slice of records (missing some entries) alongside a `nil` error, leaving the application layer unaware that a database failure occurred.

### Recommended Resolution
Always inspect the query error state immediately after iteration finishes:
```go
if err := rows.Err(); err != nil {
	return nil, fmt.Errorf("iterate rows: %w", err)
}
```

### Resolution — Fixed as recommended (plus 2 more instances found by the same pattern)

Confirmed the bug in all 4 files the review flagged. Added `rows.Err()` checks to all 4: [tenant_repo.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/tenant_repo.go) `GetAll`, [api_key_repo.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/api_key_repo.go) `GetByTenantID`, [tenant_channel_repo.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/tenant_channel_repo.go) `GetByTenantID`, [tenant_rate_limit_repo.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/tenant_rate_limit_repo.go) `GetByTenantID`.

While converting these 4 files to the `Executor` interface (Issue 1), the same `for rows.Next()` pattern with no `rows.Err()` check turned up in two files the review didn't flag: [notification_repo.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/notification_repo.go) (`GetByTenantID`, `GetByStatus`) and [notification_delivery_repo.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/notification_delivery_repo.go) (`GetByNotificationID`). Fixed those too — same bug class, same one-line fix, no reason to leave known instances of it in place.

Found but deliberately **not** fixed: `TenantRateLimitRepository.GetByChannel` swallows every `QueryRow` error (not just `pgx.ErrNoRows`) as "use default" (`if err != nil { return nil, nil }`). That's a real latent bug — a transient connection error would silently fall back to default rate limits instead of failing the request — but it's a different bug (wrong error handling, not a missing check after iteration) and outside what either review flagged. Left alone to avoid scope creep; worth its own pass later.

Verified live: ran `GET /api/v1/tenants` and the channels/rate-limits GET paths against real Postgres — all iterate-and-return correctly with `go build` confirming no regressions from the `Executor` signature change.

---

## 6. Missing CORS Configuration (Dashboard Blocker)

### Issue Description
The HTTP routing entry point [routes.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/routes/routes.go) registers standard logging and panic recovery middlewares but does not configure Cross-Origin Resource Sharing (CORS).

### Architectural Implications
1. **Browser Blocks Frontend requests:** The Flutter Web dashboard (Phase 12) runs on a different port/origin from the Go API service (e.g. `localhost:3000` vs `localhost:8080`). Without CORS headers approved by the server, modern web browsers will drop all AJAX/fetch requests to the backend, blocking dashboard operations.

### Recommended Resolution
Register Echo's built-in CORS middleware:
```go
e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
    AllowOrigins: []string{"*"}, // Restrict origins in production config
    AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "X-API-Key"},
}))
```

### Resolution — Fixed as recommended

Confirmed `routes.go` had no CORS middleware. Added the exact middleware suggested, after `echomw.Recover()` in [routes.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/routes/routes.go). `AllowOrigins: []string{"*"}` is acceptable for now since the dashboard itself is unauthenticated (no cookies/credentials involved) — tighten to the deployed dashboard's origin once Phase 12 ships one.

Verified live: `curl -i localhost:8099/health -H "Origin: http://example.com"` returned `Access-Control-Allow-Origin: *`.

