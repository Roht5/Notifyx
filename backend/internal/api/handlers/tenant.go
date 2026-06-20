package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// TenantService groups the repositories the tenant handler needs.
// DefaultGlobalRateCap seeds Tenant.GlobalRateCap on creation (from DEFAULT_GLOBAL_CAP).
// Pool is kept alongside the pool-backed repos above so handlers that need multiple
// writes to commit or roll back together (Create, UpdateChannels, UpdateRateLimits)
// can open a transaction and construct tx-scoped repos against it.
type TenantService struct {
	Tenants              *postgres.TenantRepository
	APIKeys              *postgres.APIKeyRepository
	Channels             *postgres.TenantChannelRepository
	RateLimits           *postgres.TenantRateLimitRepository
	Analytics            *postgres.AnalyticsRepository
	DefaultGlobalRateCap int
	Pool                 *pgxpool.Pool
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

type createAPIKeyResponse struct {
	ID     string `json:"id"`
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

// updateRateLimitsRequest separates the tenant-wide global cap (one value, optional —
// only updated if present) from the per-channel max_per_min array. Earlier this request
// took global_cap once per channel entry, which let the same tenant end up with N
// different "global" caps stored across N channel rows — see decisions.md.
type updateRateLimitsRequest struct {
	GlobalCap  *int `json:"global_cap,omitempty"`
	RateLimits []struct {
		Channel   string `json:"channel"`
		MaxPerMin int    `json:"max_per_min"`
	} `json:"rate_limits"`
}

func (r *updateRateLimitsRequest) Validate() error {
	if r.GlobalCap != nil && *r.GlobalCap <= 0 {
		return fmt.Errorf("global_cap must be greater than 0")
	}
	for _, rl := range r.RateLimits {
		if !isValidChannel(rl.Channel) {
			return fmt.Errorf("invalid channel %q: must be one of email, push, sms, inapp", rl.Channel)
		}
		if rl.MaxPerMin <= 0 {
			return fmt.Errorf("max_per_min must be greater than 0")
		}
	}
	return nil
}

type rateLimitsResponse struct {
	GlobalCap  int                       `json:"global_cap"`
	RateLimits []*domain.TenantRateLimit `json:"rate_limits"`
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

	rawKey, keyHash, err := generateAPIKey()
	if err != nil {
		h.log.Errorw("generate api key failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to generate api key")
	}

	// Tenant + its first API key must commit or roll back together — a tenant with no
	// usable key, or a key pointing at no tenant, are both broken states.
	var tenant *domain.Tenant
	err = postgres.WithTx(c.Request().Context(), h.svc.Pool, func(tx pgx.Tx) error {
		var txErr error
		tenant, txErr = postgres.NewTenantRepository(tx).Create(c.Request().Context(), req.Name, h.svc.DefaultGlobalRateCap)
		if txErr != nil {
			return txErr
		}
		_, txErr = postgres.NewAPIKeyRepository(tx).Create(c.Request().Context(), tenant.ID, keyHash)
		return txErr
	})
	if err != nil {
		h.log.Errorw("create tenant failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to create tenant")
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

// CreateKey POST /api/v1/tenants/:id/keys — issues a new API key for an existing
// tenant (rotation: callers create a new key, switch traffic to it, then revoke the
// old one via DeleteKey — there's no forced single-active-key constraint).
func (h *TenantHandler) CreateKey(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	if _, err := h.svc.Tenants.GetByID(c.Request().Context(), id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to verify tenant")
	}

	rawKey, keyHash, err := generateAPIKey()
	if err != nil {
		h.log.Errorw("generate api key failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to generate api key")
	}

	key, err := h.svc.APIKeys.Create(c.Request().Context(), id, keyHash)
	if err != nil {
		h.log.Errorw("store api key failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to store api key")
	}

	return c.JSON(http.StatusCreated, createAPIKeyResponse{
		ID:     key.ID.String(),
		APIKey: rawKey,
	})
}

// ListKeys GET /api/v1/tenants/:id/keys — never returns key hashes or raw keys, just
// the metadata needed to decide which key to revoke.
func (h *TenantHandler) ListKeys(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	keys, err := h.svc.APIKeys.GetByTenantID(c.Request().Context(), id)
	if err != nil {
		h.log.Errorw("list api keys failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to list api keys")
	}
	return c.JSON(http.StatusOK, keys)
}

// DeleteKey DELETE /api/v1/tenants/:id/keys/:key_id — revokes a key immediately.
// Doesn't check the key belongs to :id; api_keys.id is globally unique so the
// :id segment is purely for a RESTful URL shape, not an authorization boundary
// (these routes are already unauthenticated super-admin routes — see routes.go).
func (h *TenantHandler) DeleteKey(c echo.Context) error {
	keyID, err := uuid.Parse(c.Param("key_id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid key id")
	}

	if err := h.svc.APIKeys.Delete(c.Request().Context(), keyID); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "api key not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to revoke api key")
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

	// All channel upserts for one request commit or roll back together — a partial
	// write would leave the tenant with an inconsistent view of which channels are on.
	err = postgres.WithTx(ctx, h.svc.Pool, func(tx pgx.Tx) error {
		channels := postgres.NewTenantChannelRepository(tx)
		for _, ch := range req.Channels {
			if err := channels.Upsert(ctx, id, domain.Channel(ch.Channel), ch.Enabled); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		h.log.Errorw("upsert tenant channel failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to update channels")
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

	tenant, err := h.svc.Tenants.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to verify tenant")
	}

	// Global cap update + per-channel upserts commit or roll back together — a partial
	// write here is exactly the "N different global caps" bug this request shape was
	// designed to prevent (see updateRateLimitsRequest doc comment).
	err = postgres.WithTx(ctx, h.svc.Pool, func(tx pgx.Tx) error {
		if req.GlobalCap != nil {
			var txErr error
			tenant, txErr = postgres.NewTenantRepository(tx).UpdateGlobalRateCap(ctx, id, *req.GlobalCap)
			if txErr != nil {
				return txErr
			}
		}
		rateLimits := postgres.NewTenantRateLimitRepository(tx)
		for _, rl := range req.RateLimits {
			if err := rateLimits.Upsert(ctx, id, domain.Channel(rl.Channel), rl.MaxPerMin); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		h.log.Errorw("update rate limits failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to update rate limits")
	}

	limits, err := h.svc.RateLimits.GetByTenantID(ctx, id)
	if err != nil {
		return errResponse(c, http.StatusInternalServerError, "failed to fetch rate limits")
	}
	return c.JSON(http.StatusOK, rateLimitsResponse{GlobalCap: tenant.GlobalRateCap, RateLimits: limits})
}

// GetChannels GET /api/v1/tenants/:id/channels
func (h *TenantHandler) GetChannels(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	ctx := c.Request().Context()
	if _, err := h.svc.Tenants.GetByID(ctx, id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to verify tenant")
	}

	channels, err := h.svc.Channels.GetByTenantID(ctx, id)
	if err != nil {
		h.log.Errorw("get tenant channels failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to get channels")
	}

	return c.JSON(http.StatusOK, channels)
}

// GetRateLimits GET /api/v1/tenants/:id/rate-limits
func (h *TenantHandler) GetRateLimits(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	ctx := c.Request().Context()
	tenant, err := h.svc.Tenants.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to verify tenant")
	}

	limits, err := h.svc.RateLimits.GetByTenantID(ctx, id)
	if err != nil {
		h.log.Errorw("get tenant rate limits failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to get rate limits")
	}

	return c.JSON(http.StatusOK, rateLimitsResponse{GlobalCap: tenant.GlobalRateCap, RateLimits: limits})
}


type analyticsQueryParams struct {
	From string `query:"from"`
	To   string `query:"to"`
}

type analyticsResponse struct {
	Summary  *domain.AnalyticsSummary   `json:"summary"`
	Channels []*domain.ChannelBreakdown `json:"channels"`
	DLQTrend []*domain.DLQTrendPoint    `json:"dlq_trend"`
	From     time.Time                  `json:"from"`
	To       time.Time                  `json:"to"`
}

// Analytics GET /api/v1/tenants/:id/analytics
// from/to are optional RFC3339 timestamps defaulting to the last 30 days — mirrors the
// historyQueryParams from/to convention in notification.go.
func (h *TenantHandler) Analytics(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid tenant id")
	}

	ctx := c.Request().Context()
	if _, err := h.svc.Tenants.GetByID(ctx, id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "tenant not found")
		}
		return errResponse(c, http.StatusInternalServerError, "failed to verify tenant")
	}

	var q analyticsQueryParams
	if err := c.Bind(&q); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid query parameters")
	}

	to := time.Now()
	if q.To != "" {
		to, err = time.Parse(time.RFC3339, q.To)
		if err != nil {
			return errResponse(c, http.StatusBadRequest, "to must be RFC3339 (e.g. 2026-06-01T00:00:00Z)")
		}
	}
	from := to.AddDate(0, 0, -30)
	if q.From != "" {
		from, err = time.Parse(time.RFC3339, q.From)
		if err != nil {
			return errResponse(c, http.StatusBadRequest, "from must be RFC3339 (e.g. 2026-06-01T00:00:00Z)")
		}
	}

	summary, err := h.svc.Analytics.Summary(ctx, id, from, to)
	if err != nil {
		h.log.Errorw("analytics summary failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to compute analytics summary")
	}
	channels, err := h.svc.Analytics.ChannelBreakdown(ctx, id, from, to)
	if err != nil {
		h.log.Errorw("analytics channel breakdown failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to compute channel breakdown")
	}
	dlqTrend, err := h.svc.Analytics.DLQTrend(ctx, id, from, to)
	if err != nil {
		h.log.Errorw("analytics dlq trend failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to compute dlq trend")
	}

	return c.JSON(http.StatusOK, analyticsResponse{
		Summary:  summary,
		Channels: channels,
		DLQTrend: dlqTrend,
		From:     from,
		To:       to,
	})
}
