package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// ErrorResponse is the standard error shape for every API error in Notifyx.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// errResponse writes a consistent JSON error response.
func errResponse(c echo.Context, status int, message string) error {
	return c.JSON(status, ErrorResponse{Error: message})
}

// TenantFromContext retrieves the authenticated tenant injected by the auth middleware.
// Returns nil if called on a route that doesn't use auth middleware — that's a
// programming error, but one nil check is cheaper than a recovered panic and a 500
// with no useful context in the logs.
func TenantFromContext(c echo.Context) *domain.Tenant {
	tenant, _ := c.Get("tenant").(*domain.Tenant)
	return tenant
}

// RequireTenant fetches the authenticated tenant from context, writing a 500 response
// and returning an error if it's missing (a route registered without the auth
// middleware — a programming error, but one that should fail cleanly, not panic).
func RequireTenant(c echo.Context) (*domain.Tenant, error) {
	tenant := TenantFromContext(c)
	if tenant == nil {
		return nil, errResponse(c, http.StatusInternalServerError, "internal error: missing tenant context")
	}
	return tenant, nil
}

// PaginationParams are shared across all list endpoints.
type PaginationParams struct {
	Page  int `query:"page"`
	Limit int `query:"limit"`
}

func (p *PaginationParams) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
}

func (p *PaginationParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

// Pinger is the seam HealthHandler uses to verify a dependency is reachable, satisfied
// directly by *pgxpool.Pool and by a thin Ping adapter over *redis.Client (go-redis's
// Ping returns a *StatusCmd, not an error, so main.go wraps it before passing it in here
// to keep this package free of a go-redis import).
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves the /health endpoint used by Render for health checks. It
// actively verifies the dependencies the app cannot run without (Postgres) and the ones
// that are optional-but-configured (Redis), and reports Kafka's configured/disabled
// state without probing it directly (the producer has no cheap connectivity check).
type HealthHandler struct {
	db              Pinger
	redis           Pinger // nil when REDIS_URL isn't set
	kafkaConfigured bool
}

func NewHealthHandler(db Pinger, redis Pinger, kafkaConfigured bool) *HealthHandler {
	return &HealthHandler{db: db, redis: redis, kafkaConfigured: kafkaConfigured}
}

func (h *HealthHandler) Check(c echo.Context) error {
	ctx := c.Request().Context()
	healthy := true
	checks := map[string]string{}

	if err := h.db.Ping(ctx); err != nil {
		checks["database"] = "down: " + err.Error()
		healthy = false
	} else {
		checks["database"] = "ok"
	}

	if h.redis != nil {
		if err := h.redis.Ping(ctx); err != nil {
			checks["redis"] = "down: " + err.Error()
			healthy = false
		} else {
			checks["redis"] = "ok"
		}
	} else {
		checks["redis"] = "disabled"
	}

	if h.kafkaConfigured {
		checks["kafka"] = "configured"
	} else {
		checks["kafka"] = "disabled"
	}

	status := http.StatusOK
	overall := "ok"
	if !healthy {
		status = http.StatusServiceUnavailable
		overall = "degraded"
	}

	return c.JSON(status, map[string]any{
		"status": overall,
		"checks": checks,
	})
}
