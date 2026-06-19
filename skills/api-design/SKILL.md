# API Design Skill

Use this skill when writing any Echo handler, middleware, or route registration. Ensures every endpoint looks and behaves consistently across the entire Notifyx API.

## When to use

- Writing any handler in `internal/api/handlers/`
- Writing any middleware in `internal/api/middleware/`
- Registering routes in `internal/api/routes/`

## Handler structure

Every handler follows this exact pattern — no exceptions:

```go
func (h *NotificationHandler) Send(c echo.Context) error {
    // 1. Bind + validate request
    var req SendNotificationRequest
    if err := c.Bind(&req); err != nil {
        return errResponse(c, http.StatusBadRequest, "invalid request body")
    }
    if err := req.Validate(); err != nil {
        return errResponse(c, http.StatusUnprocessableEntity, err.Error())
    }

    // 2. Extract tenant from context (set by auth middleware)
    tenant := TenantFromContext(c)

    // 3. Call service/repository — no business logic in handlers
    result, err := h.service.Send(c.Request().Context(), tenant.ID, req)
    if err != nil {
        h.log.Error("send notification failed", zap.Error(err))
        return errResponse(c, http.StatusInternalServerError, "failed to send notification")
    }

    // 4. Return response
    return c.JSON(http.StatusCreated, result)
}
```

## Request + response structs

Define request and response structs in the same handler file. Keep them simple — no embedded structs.

```go
type SendNotificationRequest struct {
    Channel        string            `json:"channel" validate:"required,oneof=email push sms inapp"`
    RecipientID    string            `json:"recipient_id" validate:"required"`
    RecipientEmail string            `json:"recipient_email"`
    Subject        string            `json:"subject"`
    Body           string            `json:"body" validate:"required"`
    Priority       string            `json:"priority" validate:"oneof=critical high normal low"`
    IdempotencyKey string            `json:"idempotency_key"`
    ScheduledAt    *time.Time        `json:"scheduled_at"`
    Metadata       map[string]string `json:"metadata"`
}

func (r *SendNotificationRequest) Validate() error {
    if r.Channel == "email" && r.RecipientEmail == "" {
        return errors.New("recipient_email required for email channel")
    }
    return nil
}
```

## Error response format

**Always use this exact shape.** Never return a raw error string or a different JSON structure.

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code,omitempty"`
}

func errResponse(c echo.Context, status int, message string) error {
    return c.JSON(status, ErrorResponse{Error: message})
}
```

```json
// Example error response
{ "error": "recipient_email required for email channel" }
```

## HTTP status codes

| Situation | Status |
|---|---|
| Created successfully | 201 |
| Fetched successfully | 200 |
| Invalid JSON / bind error | 400 |
| Validation failed | 422 |
| Unauthenticated | 401 |
| Tenant not authorised | 403 |
| Resource not found | 404 |
| Rate limited | 429 |
| Internal error | 500 |

## Route registration

All routes registered in `internal/api/routes/routes.go`. Versioned under `/api/v1/`. Group by resource.

```go
func RegisterRoutes(e *echo.Echo, h *Handlers, authMiddleware echo.MiddlewareFunc) {
    v1 := e.Group("/api/v1")
    v1.Use(authMiddleware)

    // Notifications
    v1.POST("/notifications/send", h.Notification.Send)
    v1.POST("/notifications/batch", h.Notification.Batch)
    v1.POST("/notifications/schedule", h.Notification.Schedule)
    v1.GET("/notifications/history", h.Notification.History)
    v1.GET("/notifications/:id", h.Notification.GetByID)

    // Templates
    v1.POST("/templates", h.Template.Create)
    v1.GET("/templates", h.Template.List)
    v1.PUT("/templates/:id", h.Template.Update)
    v1.DELETE("/templates/:id", h.Template.Delete)

    // Tenants (super-admin, no tenant auth)
    admin := e.Group("/api/v1/tenants")
    admin.POST("", h.Tenant.Create)
    admin.GET("", h.Tenant.List)
    admin.PUT("/:id", h.Tenant.Update)
    admin.DELETE("/:id", h.Tenant.Delete)
    admin.GET("/:id/analytics", h.Tenant.Analytics)
    admin.PUT("/:id/channels", h.Tenant.UpdateChannels)
    admin.PUT("/:id/rate-limits", h.Tenant.UpdateRateLimits)

    // Health
    e.GET("/health", h.Health.Check)

    // WebSocket
    e.GET("/ws/connect", h.WebSocket.Connect)
}
```

## Auth middleware

Extracts `X-API-Key` header, hashes it, looks up tenant in DB, attaches to context.

```go
func AuthMiddleware(tenantRepo TenantRepository, log *zap.Logger) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            key := c.Request().Header.Get("X-API-Key")
            if key == "" {
                return errResponse(c, http.StatusUnauthorized, "missing API key")
            }
            tenant, err := tenantRepo.GetByKeyHash(c.Request().Context(), hashKey(key))
            if err != nil {
                return errResponse(c, http.StatusUnauthorized, "invalid API key")
            }
            c.Set("tenant", tenant)
            return next(c)
        }
    }
}

func TenantFromContext(c echo.Context) *domain.Tenant {
    return c.Get("tenant").(*domain.Tenant)
}
```

## Pagination

List endpoints accept `page` and `limit` query params. Always cap limit at 100.

```go
type PaginationParams struct {
    Page  int `query:"page"`
    Limit int `query:"limit"`
}

func (p *PaginationParams) Normalize() {
    if p.Page < 1 { p.Page = 1 }
    if p.Limit < 1 || p.Limit > 100 { p.Limit = 20 }
}

func (p *PaginationParams) Offset() int {
    return (p.Page - 1) * p.Limit
}
```

## Logging in handlers

Log errors only — not every request (Echo's request logger middleware handles that).

```go
h.log.Error("failed to create notification",
    zap.String("tenant_id", tenant.ID.String()),
    zap.String("channel", req.Channel),
    zap.Error(err),
)
```

Never log: email addresses, phone numbers, API keys, notification body content.
