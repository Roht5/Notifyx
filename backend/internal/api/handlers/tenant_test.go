package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// newTenantHandler builds a TenantHandler with mocked repos. Pool is left nil — handlers
// that need it (Create, UpdateChannels, UpdateRateLimits) call postgres.WithTx(ctx, pool, ...)
// directly with a *pgxpool.Pool, which has no interface seam, so they're exercised only
// indirectly here (validation / not-found short-circuits that return before WithTx is
// reached) rather than through their full success path. See final report for detail.
func newTenantHandler(tenants *mockTenantRepo, apiKeys *mockAPIKeyRepo, channels *mockChannelRepo, rateLimits *mockRateLimitRepo, analytics *mockAnalyticsRepo) *TenantHandler {
	return NewTenantHandler(&TenantService{
		Tenants:              tenants,
		APIKeys:               apiKeys,
		Channels:              channels,
		RateLimits:            rateLimits,
		Analytics:             analytics,
		DefaultGlobalRateCap:  100,
	}, newTestLogger())
}

func TestTenantHandler_List_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)

	want := []*domain.Tenant{testTenant()}
	tenants.On("GetAll", mock.Anything).Return(want, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants", "", nil)
	err := h.List(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantHandler_List_RepoError(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)

	tenants.On("GetAll", mock.Anything).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants", "", nil)
	err := h.List(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_Get_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String(), "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantHandler_Get_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, nil, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/bad", "", nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_Get_NotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+id.String(), "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_Get_RepoError(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+id.String(), "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_Update_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	tenant := testTenant()

	updated := &domain.Tenant{ID: tenant.ID, Name: "new-name"}
	tenants.On("Update", mock.Anything, tenant.ID, "new-name").Return(updated, nil)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+tenant.ID.String(), `{"name":"new-name"}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantHandler_Update_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, nil, nil)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/bad", `{"name":"x"}`, nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_Update_ValidationError(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, nil, nil)
	id := uuid.New()

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+id.String(), `{"name":""}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestTenantHandler_Update_NotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	id := uuid.New()

	tenants.On("Update", mock.Anything, id, "new-name").Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+id.String(), `{"name":"new-name"}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_Update_RepoError(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	id := uuid.New()

	tenants.On("Update", mock.Anything, id, "new-name").Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+id.String(), `{"name":"new-name"}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_Delete_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	id := uuid.New()

	tenants.On("Delete", mock.Anything, id).Return(nil)

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/"+id.String(), "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTenantHandler_Delete_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, nil, nil)

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/bad", "", nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_Delete_NotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	id := uuid.New()

	tenants.On("Delete", mock.Anything, id).Return(postgres.ErrNotFound)

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/"+id.String(), "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_Delete_RepoError(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, nil)
	id := uuid.New()

	tenants.On("Delete", mock.Anything, id).Return(assert.AnError)

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/"+id.String(), "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_CreateKey_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	apiKeys := &mockAPIKeyRepo{}
	h := newTenantHandler(tenants, apiKeys, nil, nil, nil)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	key := &domain.APIKey{ID: uuid.New(), TenantID: tenant.ID}
	apiKeys.On("Create", mock.Anything, tenant.ID, mock.AnythingOfType("string")).Return(key, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/tenants/"+tenant.ID.String()+"/keys", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.CreateKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestTenantHandler_CreateKey_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, &mockAPIKeyRepo{}, nil, nil, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/tenants/bad/keys", "", nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.CreateKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_CreateKey_TenantNotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, &mockAPIKeyRepo{}, nil, nil, nil)
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodPost, "/api/v1/tenants/"+id.String()+"/keys", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.CreateKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_CreateKey_StoreError(t *testing.T) {
	tenants := &mockTenantRepo{}
	apiKeys := &mockAPIKeyRepo{}
	h := newTenantHandler(tenants, apiKeys, nil, nil, nil)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	apiKeys.On("Create", mock.Anything, tenant.ID, mock.AnythingOfType("string")).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodPost, "/api/v1/tenants/"+tenant.ID.String()+"/keys", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.CreateKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_ListKeys_Success(t *testing.T) {
	apiKeys := &mockAPIKeyRepo{}
	h := newTenantHandler(&mockTenantRepo{}, apiKeys, nil, nil, nil)
	id := uuid.New()

	want := []*domain.APIKey{{ID: uuid.New(), TenantID: id}}
	apiKeys.On("GetByTenantID", mock.Anything, id).Return(want, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+id.String()+"/keys", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.ListKeys(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantHandler_ListKeys_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, &mockAPIKeyRepo{}, nil, nil, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/bad/keys", "", nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.ListKeys(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_ListKeys_RepoError(t *testing.T) {
	apiKeys := &mockAPIKeyRepo{}
	h := newTenantHandler(&mockTenantRepo{}, apiKeys, nil, nil, nil)
	id := uuid.New()

	apiKeys.On("GetByTenantID", mock.Anything, id).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+id.String()+"/keys", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.ListKeys(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_DeleteKey_Success(t *testing.T) {
	apiKeys := &mockAPIKeyRepo{}
	h := newTenantHandler(&mockTenantRepo{}, apiKeys, nil, nil, nil)
	tenantID, keyID := uuid.New(), uuid.New()

	apiKeys.On("Delete", mock.Anything, keyID).Return(nil)

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/"+tenantID.String()+"/keys/"+keyID.String(), "", nil)
	c.SetParamNames("id", "key_id")
	c.SetParamValues(tenantID.String(), keyID.String())
	err := h.DeleteKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTenantHandler_DeleteKey_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, &mockAPIKeyRepo{}, nil, nil, nil)
	tenantID := uuid.New()

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/"+tenantID.String()+"/keys/bad", "", nil)
	c.SetParamNames("id", "key_id")
	c.SetParamValues(tenantID.String(), "bad")
	err := h.DeleteKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_DeleteKey_NotFound(t *testing.T) {
	apiKeys := &mockAPIKeyRepo{}
	h := newTenantHandler(&mockTenantRepo{}, apiKeys, nil, nil, nil)
	tenantID, keyID := uuid.New(), uuid.New()

	apiKeys.On("Delete", mock.Anything, keyID).Return(postgres.ErrNotFound)

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/"+tenantID.String()+"/keys/"+keyID.String(), "", nil)
	c.SetParamNames("id", "key_id")
	c.SetParamValues(tenantID.String(), keyID.String())
	err := h.DeleteKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_DeleteKey_RepoError(t *testing.T) {
	apiKeys := &mockAPIKeyRepo{}
	h := newTenantHandler(&mockTenantRepo{}, apiKeys, nil, nil, nil)
	tenantID, keyID := uuid.New(), uuid.New()

	apiKeys.On("Delete", mock.Anything, keyID).Return(assert.AnError)

	c, rec := newCtx(http.MethodDelete, "/api/v1/tenants/"+tenantID.String()+"/keys/"+keyID.String(), "", nil)
	c.SetParamNames("id", "key_id")
	c.SetParamValues(tenantID.String(), keyID.String())
	err := h.DeleteKey(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_UpdateChannels_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, &mockChannelRepo{}, nil, nil)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/bad/channels", `{"channels":[]}`, nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.UpdateChannels(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_UpdateChannels_ValidationError(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, &mockChannelRepo{}, nil, nil)
	id := uuid.New()

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+id.String()+"/channels", `{"channels":[{"channel":"bogus","enabled":true}]}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.UpdateChannels(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestTenantHandler_UpdateChannels_TenantNotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, &mockChannelRepo{}, nil, nil)
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+id.String()+"/channels", `{"channels":[{"channel":"email","enabled":true}]}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.UpdateChannels(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_UpdateRateLimits_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, &mockRateLimitRepo{}, nil)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/bad/rate-limits", `{"rate_limits":[]}`, nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.UpdateRateLimits(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_UpdateRateLimits_ValidationError(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, &mockRateLimitRepo{}, nil)
	id := uuid.New()

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+id.String()+"/rate-limits", `{"global_cap":-1}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.UpdateRateLimits(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestTenantHandler_UpdateRateLimits_TenantNotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, &mockRateLimitRepo{}, nil)
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodPut, "/api/v1/tenants/"+id.String()+"/rate-limits", `{"rate_limits":[{"channel":"email","max_per_min":10}]}`, nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.UpdateRateLimits(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_GetChannels_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	channels := &mockChannelRepo{}
	h := newTenantHandler(tenants, nil, channels, nil, nil)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	want := []*domain.TenantChannel{{ID: uuid.New(), TenantID: tenant.ID, Channel: domain.ChannelEmail, Enabled: true}}
	channels.On("GetByTenantID", mock.Anything, tenant.ID).Return(want, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String()+"/channels", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.GetChannels(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantHandler_GetChannels_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, &mockChannelRepo{}, nil, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/bad/channels", "", nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.GetChannels(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_GetChannels_TenantNotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, &mockChannelRepo{}, nil, nil)
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+id.String()+"/channels", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.GetChannels(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_GetChannels_RepoError(t *testing.T) {
	tenants := &mockTenantRepo{}
	channels := &mockChannelRepo{}
	h := newTenantHandler(tenants, nil, channels, nil, nil)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	channels.On("GetByTenantID", mock.Anything, tenant.ID).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String()+"/channels", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.GetChannels(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_GetRateLimits_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	rateLimits := &mockRateLimitRepo{}
	h := newTenantHandler(tenants, nil, nil, rateLimits, nil)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	want := []*domain.TenantRateLimit{{ID: uuid.New(), TenantID: tenant.ID, Channel: domain.ChannelEmail, MaxPerMin: 50}}
	rateLimits.On("GetByTenantID", mock.Anything, tenant.ID).Return(want, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String()+"/rate-limits", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.GetRateLimits(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantHandler_GetRateLimits_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, &mockRateLimitRepo{}, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/bad/rate-limits", "", nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.GetRateLimits(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_GetRateLimits_TenantNotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, &mockRateLimitRepo{}, nil)
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+id.String()+"/rate-limits", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.GetRateLimits(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_GetRateLimits_RepoError(t *testing.T) {
	tenants := &mockTenantRepo{}
	rateLimits := &mockRateLimitRepo{}
	h := newTenantHandler(tenants, nil, nil, rateLimits, nil)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	rateLimits.On("GetByTenantID", mock.Anything, tenant.ID).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String()+"/rate-limits", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.GetRateLimits(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTenantHandler_Analytics_Success(t *testing.T) {
	tenants := &mockTenantRepo{}
	analytics := &mockAnalyticsRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, analytics)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	analytics.On("Summary", mock.Anything, tenant.ID, mock.Anything, mock.Anything).Return(&domain.AnalyticsSummary{TotalSent: 10}, nil)
	analytics.On("ChannelBreakdown", mock.Anything, tenant.ID, mock.Anything, mock.Anything).Return([]*domain.ChannelBreakdown{}, nil)
	analytics.On("DLQTrend", mock.Anything, tenant.ID, mock.Anything, mock.Anything).Return([]*domain.DLQTrendPoint{}, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String()+"/analytics", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.Analytics(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantHandler_Analytics_BadID(t *testing.T) {
	h := newTenantHandler(&mockTenantRepo{}, nil, nil, nil, &mockAnalyticsRepo{})

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/bad/analytics", "", nil)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.Analytics(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_Analytics_TenantNotFound(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, &mockAnalyticsRepo{})
	id := uuid.New()

	tenants.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+id.String()+"/analytics", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Analytics(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTenantHandler_Analytics_BadFromTo(t *testing.T) {
	tenants := &mockTenantRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, &mockAnalyticsRepo{})
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String()+"/analytics?to=not-a-date", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.Analytics(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTenantHandler_Analytics_SummaryError(t *testing.T) {
	tenants := &mockTenantRepo{}
	analytics := &mockAnalyticsRepo{}
	h := newTenantHandler(tenants, nil, nil, nil, analytics)
	tenant := testTenant()

	tenants.On("GetByID", mock.Anything, tenant.ID).Return(tenant, nil)
	analytics.On("Summary", mock.Anything, tenant.ID, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/tenants/"+tenant.ID.String()+"/analytics", "", nil)
	c.SetParamNames("id")
	c.SetParamValues(tenant.ID.String())
	err := h.Analytics(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

var _ = time.Now // keep time import used if future tests need it
