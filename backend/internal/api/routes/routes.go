package routes

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/api/middleware"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// Handlers bundles all handler types — added to as new phases are implemented.
type Handlers struct {
	Tenant       *handlers.TenantHandler
	Health       *handlers.HealthHandler
	Notification *handlers.NotificationHandler
}

// Setup creates the Echo instance, registers global middleware, and wires all routes.
func Setup(h *Handlers, apiKeyRepo *postgres.APIKeyRepository, log *logger.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Global middleware — runs on every request.
	e.Use(echomw.RequestID())            // adds X-Request-ID header
	e.Use(middleware.RequestLogger(log)) // structured Zap request logging
	e.Use(echomw.Recover())              // recover from panics, return 500

	authMW := middleware.Auth(apiKeyRepo, log)

	// Health — no auth required (used by Render's health check probe).
	e.GET("/health", h.Health.Check)

	// Tenant routes — no auth (super-admin, open for portfolio purposes per decisions.md).
	tenants := e.Group("/api/v1/tenants")
	tenants.POST("", h.Tenant.Create)
	tenants.GET("", h.Tenant.List)
	tenants.GET("/:id", h.Tenant.Get)
	tenants.PUT("/:id", h.Tenant.Update)
	tenants.DELETE("/:id", h.Tenant.Delete)
	tenants.PUT("/:id/channels", h.Tenant.UpdateChannels)
	tenants.PUT("/:id/rate-limits", h.Tenant.UpdateRateLimits)

	// Authenticated API — all routes below require X-API-Key.
	api := e.Group("/api/v1", authMW)

	notifications := api.Group("/notifications")
	notifications.POST("/send", h.Notification.Send)
	notifications.POST("/batch", h.Notification.Batch)
	notifications.GET("/history", h.Notification.History)
	notifications.GET("/:id", h.Notification.Get)

	return e
}
