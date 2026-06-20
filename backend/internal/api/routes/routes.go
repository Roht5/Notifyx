package routes

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/api/middleware"
	"github.com/rohit-bagade/notifyx/internal/metrics"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/internal/ws"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// Handlers bundles all handler types — added to as new phases are implemented.
type Handlers struct {
	Tenant       *handlers.TenantHandler
	Health       *handlers.HealthHandler
	Notification *handlers.NotificationHandler
	Template     *handlers.TemplateHandler
	WS           *ws.Handler
}

// Setup creates the Echo instance, registers global middleware, and wires all routes.
// env is config.Config.Env ("development" | "production") — it only affects the CORS policy.
func Setup(h *Handlers, apiKeyRepo *postgres.APIKeyRepository, log *logger.Logger, env string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Global middleware — runs on every request.
	e.Use(echomw.RequestID())            // adds X-Request-ID header
	e.Use(middleware.RequestLogger(log)) // structured Zap request logging
	e.Use(echomw.Recover())              // recover from panics, return 500
	e.Use(metrics.EchoMiddleware())      // Prometheus request latency histogram

	// CORS — needed once the Flutter Web dashboard (Phase 12) calls this API from a
	// different origin. In development, CORS is fully open (any origin, any method/header)
	// so the Flutter web dev server (which runs on a random/changing localhost port) never
	// hits a preflight rejection. In production, still "*" for now since the dashboard is
	// unauthenticated (see decisions.md) — tighten to the deployed dashboard origin once
	// one exists, but development stays permissive regardless.
	if env == "development" {
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE, echo.OPTIONS},
			AllowHeaders: []string{"*"},
		}))
	} else {
		e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "X-API-Key"},
		}))
	}

	authMW := middleware.Auth(apiKeyRepo, log)

	// Health — no auth required (used by Render's health check probe).
	e.GET("/health", h.Health.Check)

	// Metrics — no auth required (scraped by Prometheus, see docker/prometheus.yml).
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// In-app WebSocket connect — auth is via apiKey query param, not the X-API-Key header
	// middleware, since browsers' WebSocket API can't set custom handshake headers. Auth is
	// handled inside ws.Handler.Connect, so this is registered outside the authMW group.
	e.GET("/ws/connect", h.WS.Connect)

	// Tenant routes — no auth (super-admin, open for portfolio purposes per decisions.md).
	tenants := e.Group("/api/v1/tenants")
	tenants.POST("", h.Tenant.Create)
	tenants.GET("", h.Tenant.List)
	tenants.GET("/:id", h.Tenant.Get)
	tenants.PUT("/:id", h.Tenant.Update)
	tenants.DELETE("/:id", h.Tenant.Delete)
	tenants.PUT("/:id/channels", h.Tenant.UpdateChannels)
	tenants.GET("/:id/channels", h.Tenant.GetChannels)
	tenants.PUT("/:id/rate-limits", h.Tenant.UpdateRateLimits)
	tenants.GET("/:id/rate-limits", h.Tenant.GetRateLimits)
	tenants.GET("/:id/analytics", h.Tenant.Analytics)
	tenants.POST("/:id/keys", h.Tenant.CreateKey)
	tenants.GET("/:id/keys", h.Tenant.ListKeys)
	tenants.DELETE("/:id/keys/:key_id", h.Tenant.DeleteKey)

	// Authenticated API — all routes below require X-API-Key.
	api := e.Group("/api/v1", authMW)

	notifications := api.Group("/notifications")
	notifications.POST("/send", h.Notification.Send)
	notifications.POST("/schedule", h.Notification.Schedule)
	notifications.POST("/batch", h.Notification.Batch)
	notifications.GET("/history", h.Notification.History)
	notifications.GET("/:id", h.Notification.Get)

	templates := api.Group("/templates")
	templates.POST("", h.Template.Create)
	templates.GET("", h.Template.List)
	templates.GET("/:id", h.Template.Get)
	templates.PUT("/:id", h.Template.Update)
	templates.DELETE("/:id", h.Template.Delete)

	return e
}
