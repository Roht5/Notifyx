package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// Auth returns an Echo middleware that validates the X-API-Key header and
// injects the authenticated *domain.Tenant into the request context.
func Auth(apiKeyRepo *postgres.APIKeyRepository, log *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get("X-API-Key")
			if key == "" {
				return c.JSON(http.StatusUnauthorized, handlers.ErrorResponse{Error: "missing X-API-Key header"})
			}

			keyHash := handlers.HashAPIKey(key)
			tenant, err := apiKeyRepo.GetTenantByKeyHash(c.Request().Context(), keyHash)
			if err != nil {
				if errors.Is(err, postgres.ErrNotFound) {
					return c.JSON(http.StatusUnauthorized, handlers.ErrorResponse{Error: "invalid API key"})
				}
				log.Errorw("auth middleware db error", "error", err)
				return c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{Error: "authentication failed"})
			}

			// Store the tenant in Echo's context so handlers can retrieve it via TenantFromContext().
			c.Set("tenant", tenant)
			return next(c)
		}
	}
}
