# Phase 1 Foundation — Architectural & Implementation Review

This document contains a detailed analysis, architectural critique, and recommended improvements for the foundational components implemented in **Phase 1** of the Notifyx service.

> **Status: all 7 issues addressed (2026-06-19).** 5 fixed as recommended, 1 fixed with a
> different approach than recommended (see its Resolution — same underlying problem,
> different tradeoff), 1 acknowledged as valid but deferred to Phase 13 with a tracked
> checklist item, since there's no deploy pipeline yet to attach the fix to. One claim in
> the original review (pgxpool's default connection count) was checked against the
> vendored source and found incorrect — noted in its Resolution rather than silently
> corrected. Every code fix was verified against a real local Postgres instance (and for
> the logging/HTTP-status fixes, a running instance hit with real requests), not just
> `go build`. Full reasoning for each is also in `planning/decisions.md`.

---

## 1. Database Schema & Normalization (Critical Design Flaw)

### Issue Description
The rate-limiting database schema violates Third Normal Form (3NF) by storing a tenant-wide limit in a channel-specific configuration table.

```sql
-- file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/migrations/001_initial_schema.up.sql
CREATE TABLE IF NOT EXISTS tenant_rate_limits (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    channel      TEXT        NOT NULL,
    max_per_min  INT         NOT NULL DEFAULT 60,
    global_cap   INT         NOT NULL DEFAULT 300, -- <--- Tenant-wide setting stored per-channel
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, channel)
);
```

### Architectural Implications
1. **Data Inconsistency Risk:** If a tenant updates their email channel limit with a `global_cap` of `500` and later updates their SMS channel limit with a `global_cap` of `600`, the database stores conflicting global thresholds for the same tenant.
2. **Rate Limiting Logic Bug:** In [limiter.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/ratelimit/limiter.go#L87), the `resolveLimits` method checks rate limits for the current channel. If the tenant has custom email limits but uses default SMS limits, the email check resolves the custom `global_cap` from the DB, but the SMS check falls back to the server-wide default `global_cap` because there is no SMS row in the table.

### Recommended Resolution
Normalize the database schema:
* Remove `global_cap` from the `tenant_rate_limits` table.
* Store `global_cap` directly in the `tenants` table, or create a separate `tenant_settings` table for global configurations.
* Modify the `TenantRateLimitRepository` and `Limiter` queries to resolve the global cap from the tenant level and channel limits from the channel level.

### Resolution — Fixed as recommended

Took the first option (column on `tenants`, not a separate `tenant_settings` table — one
scalar value didn't justify a whole new table). Concretely:

* `tenants.global_rate_cap INT NOT NULL DEFAULT 300` added; `tenant_rate_limits.global_cap`
  dropped entirely.
* `domain.Tenant` gained `GlobalRateCap`; `domain.TenantRateLimit` lost `GlobalCap`.
* `Limiter.resolveLimits` now fetches the per-channel max from `TenantRateLimitRepository`
  and the global cap from `TenantRepository.GetByID` independently — the bug is structurally
  impossible now since there's only one place the global cap can live.
* `PUT /tenants/:id/rate-limits` request shape changed: `global_cap` moved from inside each
  array entry to a single optional top-level field. Response now returns
  `{"global_cap": ..., "rate_limits": [...]}`.
* `DEFAULT_GLOBAL_CAP` still seeds new tenants' `global_rate_cap` at creation time (threaded
  through `TenantRepository.Create` explicitly, rather than left as a SQL column default,
  so the env var still has a real effect).

**Verified:** real local Postgres round-trip (up → down → up) confirmed the schema; then ran
the actual server against it and hit the live HTTP endpoints — created a tenant (confirmed
default `global_rate_cap: 300`), updated it to 500 via `PUT .../rate-limits`, confirmed `GET`
on the tenant reflected the new value.

---

## 2. Clean Architecture Violation in Domain Models

### Issue Description
The domain entity `Notification` is decorated with JSON struct tags (e.g. `json:"id"`, `json:"tenant_id,omitempty"`).

```go
// file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/domain/notification.go
type Notification struct {
	ID             uuid.UUID      `json:"id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	Channel        Channel        `json:"channel"`
	Priority       Priority       `json:"priority"`
	Status         Status         `json:"status"`
	// ...
}
```

### Architectural Implications
1. **Leaky Abstraction:** Adding transport serialization tags (`json:"..."`) directly to domain entities binds the core business rules layer to the HTTP protocol.
2. **Inconsistency:** Other domain structs (such as `Tenant` and `NotificationTemplate`) do not have these serialization tags.
3. **API Contracts Tied to Database Models:** If the API response structure changes, you are forced to edit the core domain entity, violating the Single Responsibility Principle.

### Recommended Resolution
* Remove all serialization tags from files in the [internal/domain](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/domain) package.
* Define dedicated request/response Data Transfer Objects (DTOs) in the [api/handlers](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers) package.
* Map DTOs to/from domain entities inside the API layer.

### Resolution — Fixed differently than recommended (disagree with the DTO layer)

The inconsistency this issue points at is real: `Notification`/`NotificationDelivery` had
`json` tags, but `Tenant`, `TenantChannel`, `TenantRateLimit`, `APIKey`,
`NotificationTemplate`, and `DLQMessage` didn't — they were marshaling with Go's default
field-name encoding (capitalized keys like `"ID"`, `"Name"`), while everything else in the
API is snake_case. That's a genuine API consistency bug, fixed by adding `json` tags to
every remaining domain struct (`APIKey.KeyHash` got `json:"-"` instead, since a key hash
should never be returned over the API regardless).

Disagreed with the *prescribed fix* — a DTO per domain entity, mapped explicitly in the
handler layer. That's the textbook Clean Architecture answer, but for a solo, single-binary
portfolio project with no other consumer of the domain layer, it's a parallel struct plus a
mapping function for every entity any endpoint returns, growing every remaining phase, in
exchange for a decoupling benefit (domain insulated from transport) that has no one to
benefit from here. Chose the lighter fix: keep the domain structs as the wire shape, make
that shape consistent everywhere. Logged in `planning/decisions.md` so this can be
revisited if the calculus changes (e.g. if a second API consumer with different shape needs
ever shows up).

---

## 3. Incomplete Telemetry in Request Logging Middleware

### Issue Description
The HTTP logging middleware [request_logger.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/middleware/request_logger.go) fails to capture and log handler errors.

```go
func RequestLogger(log *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c) // <--- Error is captured but not logged here
			latency := time.Since(start)

			req := c.Request()
			res := c.Response()

			log.Infow("http request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", res.Status,
				"latency_ms", latency.Milliseconds(),
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
			)

			return err
		}
	}
}
```

### Architectural Implications
1. **Missing Telemetry:** Internal server errors (500s) or validation errors (422s) will be logged as simple HTTP requests, but the actual error message or stack trace returned by the handler will never be printed to the logs.
2. **Incorrect Status Code Logging:** Because Echo's central error handler sets the HTTP status code *after* the middleware returns, `res.Status` will be logged as its pre-processed value (often `200` or `0`) instead of the true final HTTP status code.

### Recommended Resolution
Check if `err != nil`, call Echo's default HTTP error handler explicitly to commit the correct status code before logging, and append the error content to the logs:

```go
if err != nil {
    c.Error(err) // Commit final status code
    log.Errorw("http request failed",
        "method", req.Method,
        "path", req.URL.Path,
        "status", res.Status,
        "latency_ms", latency.Milliseconds(),
        "error", err.Error(),
    )
} else {
    log.Infow("http request",
        "method", req.Method,
        "path", req.URL.Path,
        "status", res.Status,
        "latency_ms", latency.Milliseconds(),
    )
}
```

### Resolution — Fixed as recommended, verified against the actual framework behavior

Implemented essentially the suggested fix (`c.Error(err)` before logging, branch to
`Errorw`/`Infow`). Before writing it, verified the claim about *why* this happens by reading
the vendored `echo@v4.15.4` source directly rather than taking it on faith:

* `Echo.ServeHTTP` only calls `e.HTTPErrorHandler(err, c)` — the thing that actually writes
  status + body — *after* the entire middleware chain returns (`echo.go:688`).
* Every handler in this codebase happens to call `c.JSON`/`c.NoContent` itself, so in
  practice `res.Status` was already correct for application-level errors. The actual gap is
  framework-generated errors that never reach a handler at all: 404s and 405s from the
  router. Those arrive at `RequestLogger` as a bare, uncommitted error.
* Confirmed `c.Error(err)` is safe to call unconditionally: `DefaultHTTPErrorHandler` checks
  `if c.Response().Committed { return }` at the top (`echo.go:440`), so calling it on an
  already-committed response (the normal case) is a no-op.

**Verified live**, not just by re-reading the diff: ran the actual server, curled a
nonexistent path. Before the fix this would have logged `INFO`, `status: 200`, no error
field. After:
```json
{"level":"ERROR","method":"GET","path":"/this-route-does-not-exist","status":404,"error":"code=404, message=Not Found"}
```

---

## 4. Lack of Configuration Validation and Safe Defaults

### Issue Description
The configuration loader in [config.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/config/config.go) parses environment variables but fails to validate them.

### Architectural Implications
1. **Deferred Failures:** Typos or missing parameters (e.g. leaving `DATABASE_URL` empty) will not prevent the configuration from loading. Instead, the application will crash later with confusing errors during pool initialization or migration execution.
2. **Production Risk (Connection Pool Capping):** In [db.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/repository/postgres/db.go), the database connection pool is created using the raw connection string without explicit limit configuration. By default, `pgxpool` allows up to ~90-100 connections. On low-tier or free-tier databases (e.g. Render, Supabase) capped at 10-20 connections, this will lead to database connection exhaustion under load.

### Recommended Resolution
* Add a `Validate() error` method to the `Config` struct to check for required fields (`DATABASE_URL`, `APP_ENV`).
* Explicitly configure connection pool thresholds in code:
  ```go
  cfg.MaxConns = 10
  cfg.MinConns = 2
  ```

### Resolution — Fixed as recommended; one factual claim in the issue was wrong

Added `Config.Validate()` (checks `DATABASE_URL` is set, `APP_ENV` is a recognized value,
`DB_MAX_CONNS`/`DB_MIN_CONNS` are sane), called from `Load()` so every existing caller gets
it for free. Added `DB_MAX_CONNS`/`DB_MIN_CONNS` env vars (default 10/0), wired into
`pgxpool.Config.MaxConns`/`MinConns` in `NewPool`.

One correction: this issue states "`pgxpool` allows up to ~90-100 connections" by default.
Checked the vendored `pgx/v5@v5.10.0` source (`pgxpool/pool.go:19,375-385`) — the actual
default is `max(4, runtime.NumCPU())`, nowhere near 90-100. The underlying point (an
implicit, CPU-count-derived pool size is less predictable than an explicit one, and
free-tier Postgres plans do cap connections low) still held, so fixed it anyway — just
correcting the specific number rather than repeating it. Also used `MinConns=0` rather than
the suggested `2`: this app is mostly idle between requests and Render's free tier
cold-starts anyway, so eagerly holding 2 idle connections open buys little.

**Verified:** ran the built binary with `DATABASE_URL` unset (failed fast with the expected
message), with `APP_ENV=staging` (failed fast), and with a real Postgres connection (started
normally, applied migrations, served requests).

---

## 5. Migrations Run on App Startup Loop

### Issue Description
The database migrations are invoked via `postgres.RunMigrations()` during the main application startup in [main.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/cmd/server/main.go#L44).

### Architectural Implications
1. **Deployment Race Conditions:** When scaling horizontally (e.g., rolling out multiple instances/replicas on Render or Kubernetes), multiple app instances will start concurrently. They will all attempt to run database migrations at the same time.
2. **Locking Failures:** While `golang-migrate` uses Postgres advisory locks to prevent simultaneous writes, the blocked startup containers will often time out waiting for the lock, leading to failed deployments.

### Recommended Resolution
Decouple migrations from application startup. Run migrations as a pre-deployment step/job in your CI/CD pipeline or Render's pre-deploy command handler.

### Resolution — Valid, but deferred to Phase 13 rather than fixed now

The underlying mechanism (advisory-lock contention + startup-timeout risk under
concurrent instances) is correctly described and is real general-purpose advice. Checked
it against what this project actually plans to deploy, though:
`requirements/tech-stack.md` specifies "Render (1 Web Service + 1 PostgreSQL)" and
`non-functional-requirements.md` describes Notifyx as a single deployable service with no
horizontal scaling planned. The specific race this issue describes — multiple instances
starting concurrently — doesn't apply to that topology.

There's also no Dockerfile, `render.yaml`, or pre-deploy pipeline yet (that's Phase 13,
which hasn't started) for a "run migrations separately" mechanism to attach to — building
one now would be solving a problem against infrastructure that doesn't exist. Added a
tracked checklist item to `planning/milestones.md` Phase 13 instead: when the Render config
is actually built, point its Pre-Deploy Command at a migration-only run and remove the
`RunMigrations` call from `main()`.

---

## 6. Outdated and Inconsistent API Key Documentation

### Issue Description
In [tenant.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/domain/tenant.go#L39), the struct documentation states that the system stores a `bcrypt` hash of the API key:
```go
// The raw key is only shown once at creation — we store only the bcrypt hash.
```
However, the actual database schema and [apikey.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/api/handlers/apikey.go) implementation use a standard `SHA-256` hash.

### Architectural Implications
1. **Developer Confusion:** A developer looking at domain models might assume that slow hashing (`bcrypt`) is being performed during authentication lookups.
2. **Incorrect Hashing Assumptions:** Using `bcrypt` for API keys is an anti-pattern because it is computationally slow and non-deterministic, preventing query indexes (`WHERE key_hash = ?`) from working. The implementation correctly chose `SHA-256`, but the code comment incorrectly indicates `bcrypt`.

### Recommended Resolution
Update the comment in [tenant.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/internal/domain/tenant.go#L39) to align with reality:
```diff
-// The raw key is only shown once at creation — we store only the bcrypt hash.
+// The raw key is only shown once at creation — we store only the SHA-256 hash.
```

### Resolution — Fixed as recommended

Comment updated, with the "why SHA-256 not bcrypt" reasoning folded in (deterministic
hash needed for `WHERE key_hash = ?` lookups; bcrypt salts per-call and can't be looked up
this way). The implementation was always correct — only the comment was stale.

---

## 7. Hardcoded and Non-Configurable Logging Levels

### Issue Description
In [logger.go](file:///Users/rohitbagade/Rohit/AEF/Career/Notifyx/backend/pkg/logger/logger.go#L18), log verbosity levels are tightly coupled to the `APP_ENV` environment variable, hardcoding `Info` for production and `Debug` for development:
```go
if env == "production" {
    cfg := zap.NewProductionConfig() // info level
    ...
} else {
    cfg := zap.NewDevelopmentConfig() // debug level
    ...
}
```

### Architectural Implications
1. **Lack of Dynamic Operational Control:** There is no way to dynamically adjust log verbosity in production without modifying the source code or changing the global environment mode. If an incident occurs in production, operators cannot increase the log level to `Debug` or `Warn` to diagnose the issue.
2. **Noisy Production Environment:** In a high-throughput notification service, hardcoding log levels can lead to either massive ingestion charges due to excessively verbose logging or missing crucial warning/debug output.

### Recommended Resolution
Read a dedicated `LOG_LEVEL` environment variable (e.g. via `config.go`) and parse it using `zapcore.ParseLevel` to initialize the logger with custom, runtime-configurable verbosity levels:
```go
var level zapcore.Level
if err := level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))); err != nil {
    level = zapcore.InfoLevel // fallback default
}
```

### Resolution — Fixed as recommended, with one deliberate change: fail fast instead of silently falling back

Added `LOG_LEVEL` to `config.go`, threaded into `logger.New(env, levelOverride string)`.
One difference from the suggested snippet: an invalid value returns an error instead of
silently falling back to Info. Silently swallowing a typo'd `LOG_LEVEL` means you might
think you've turned on debug logging during an incident and be quietly wrong about it —
worse than a startup failure that's immediately visible. Consistent with `Config.Validate()`
failing fast elsewhere (Issue 4) rather than letting bad config degrade silently.

**Verified live:** `LOG_LEVEL=bogus` failed startup immediately with
`invalid LOG_LEVEL "bogus": unrecognized level: "bogus"`. `LOG_LEVEL=warn` against a running
instance suppressed the INFO-level "http request" line for a successful `/health` call
while still logging the ERROR-level line for a 404 — confirming the override actually
takes effect, not just that it parses.

