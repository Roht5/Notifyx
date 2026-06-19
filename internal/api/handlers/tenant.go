package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// TenantService groups the repositories the tenant handler needs.
type TenantService struct {
	Tenants    *postgres.TenantRepository
	APIKeys    *postgres.APIKeyRepository
	Channels   *postgres.TenantChannelRepository
	RateLimits *postgres.TenantRateLimitRepository
}

// TenantHandler handles all /api/v1/tenants routes.
type TenantHandler struct {
	svc *TenantService
	log *logger.Logger
}

func NewTenantHandler(svc *TenantService, log *logger.Logger) *TenantHandler {
	return &TenantHandler{svc: svc, log: log}
}

// --- request / response types ---

type createTenantRequest struct {
	Name string `json:"name"`
}

func (r *createTenantRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

type createTenantResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	APIKey string `json:"api_key"` // raw key — shown once only
}

type updateTenantRequest struct {
	Name string `json:"name"`
}

type updateChannelsRequest struct {
	Channels []struct {
		Channel string `json:"channel"`
		Enabled bool   `json:"enabled"`
	} `json:"channels"`
}

func (r *updateChannelsRequest) Validate() error {
	for _, ch := range r.Channels {
		if !isValidChannel(ch.Channel) {
			return fmt.Errorf("invalid channel %q: must be one of email, push, sms, inapp", ch.Channel)
		}
	}
	return nil
}

type updateRateLimitsRequest struct {
	RateLimits []struct {
		Channel   string `json:"channel"`
		MaxPerMin int    `json:"max_per_min"`
		GlobalCap int    `json:"global_cap"`
	} `json:"rate_limits"`
}

func (r *updateRateLimitsRequest) Validate() error {
	for _, rl := range r.RateLimits {
		if !isValidChannel(rl.Channel) {
			return fmt.Errorf("invalid channel %q: must be one of email, push, sms, inapp", rl.Channel)
		}
		if rl.MaxPerMin <= 0 {
			return fmt.Errorf("max_per_min must be greater than 0")
		}
		if rl.GlobalCap <= 0 {
			return fmt.Errorf("global_cap must be greater than 0")
		}
	}
	return nil
}

func isValidChannel(ch string) bool {
	switch domain.Channel(ch) {
	case domain.ChannelEmail, domain.ChannelPush, domain.ChannelSMS, domain.ChannelInApp:
		return true
	}
	return false
}

// --- handlers ---

// Create POST /api/v1/tenants
func (h *TenantHandler) Create(c echo.Context) error {
	var req createTenantRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return errResponse(c, http.StatusUnprocessableEntity, err.Error())
	}

	tenant, err := h.svc.Tenants.Create(c.Request().Context(), req.Name)
	if err != nil {
		h.log.Errorw("create tenant failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to create tenant")
	}

	rawKey, keyHash, err := generateAPIKey()
	if err != nil {
		h.log.Errorw("generate api key failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to generate api key")
	}

	if _, err := h.svc.APIKeys.Create(c.Request().Context(), tenant.ID, keyHash); err != nil {
		h.log.Errorw("store api key failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to store api key")
	}

	return c.JSON(http.StatusCreated, createTenantResponse{
		ID:     tenant.ID.String(),
		Name:   tenant.Name,
		APIKey: rawKey,
	})
}

// List GET /api/v1/tenants
func (h *TenantHandler) List(c echo.Context) error {
	tenants, err := h.svc.Tenants.GetAll(c.Request().Context())
	if err != nil {
		h.log.Errorw("list tenants failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to list tenants")
	}
	return c.JSON(http.StatusOK, tenants)
}

// Get GET /api/v1/tenants/:id
func (h *TenantHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	tenant, err := h.svc.Tenants.GetByID(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to get tenant")
	}
	return c.JSON(http.StatusOK, tenant)
}

// Update PUT /api/v1/tenants/:id
func (h *TenantHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	var req updateTenantRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if req.Name == "" {
		return errResponse(c, http.StatusUnprocessableEntity, "name is required")
	}

	tenant, err := h.svc.Tenants.Update(c.Request().Context(), id, req.Name)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to update tenant")
	}
	return c.JSON(http.StatusOK, tenant)
}

// Delete DELETE /api/v1/tenants/:id
func (h *TenantHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	if err := h.svc.Tenants.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to delete tenant")
	}
	return c.NoContent(http.StatusNoContent)
}

// UpdateChannels PUT /api/v1/tenants/:id/channels
func (h *TenantHandler) UpdateChannels(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	var req updateChannelsRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return errResponse(c, http.StatusUnprocessableEntity, err.Error())
	}

	ctx := c.Request().Context()

	// Verify the tenant exists before upserting child rows.
	if _, err := h.svc.Tenants.GetByID(ctx, id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to verify tenant")
	}

	for _, ch := range req.Channels {
		if err := h.svc.Channels.Upsert(ctx, id, domain.Channel(ch.Channel), ch.Enabled); err != nil {
			h.log.Errorw("upsert tenant channel failed", "error", err)
			return errResponse(c, http.StatusInternalServerError, "failed to update channels")
		}
	}

	channels, err := h.svc.Channels.GetByTenantID(ctx, id)
	if err != nil {
		return errResponse(c, http.StatusInternalServerError, "failed to fetch channels")
	}
	return c.JSON(http.StatusOK, channels)
}

// UpdateRateLimits PUT /api/v1/tenants/:id/rate-limits
func (h *TenantHandler) UpdateRateLimits(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	var req updateRateLimitsRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return errResponse(c, http.StatusUnprocessableEntity, err.Error())
	}

	ctx := c.Request().Context()

	// Verify the tenant exists before upserting child rows.
	if _, err := h.svc.Tenants.GetByID(ctx, id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to verify tenant")
	}

	for _, rl := range req.RateLimits {
		if err := h.svc.RateLimits.Upsert(ctx, id, domain.Channel(rl.Channel), rl.MaxPerMin, rl.GlobalCap); err != nil {
			h.log.Errorw("upsert rate limit failed", "error", err)
			return errResponse(c, http.StatusInternalServerError, "failed to update rate limits")
		}
	}

	limits, err := h.svc.RateLimits.GetByTenantID(ctx, id)
	if err != nil {
		return errResponse(c, http.StatusInternalServerError, "failed to fetch rate limits")
	}
	return c.JSON(http.StatusOK, limits)
}
