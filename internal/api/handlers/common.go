package handlers

import (
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
// Panics if called on a route that doesn't use auth middleware — that's a programming error.
func TenantFromContext(c echo.Context) *domain.Tenant {
	return c.Get("tenant").(*domain.Tenant)
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

// HealthHandler serves the /health endpoint used by Render for health checks.
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

func (h *HealthHandler) Check(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
