package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger(t *testing.T) *logger.Logger {
	t.Helper()
	l, err := logger.New("test", "error")
	require.NoError(t, err)
	return l
}

type mockAPIKeyRepo struct{ mock.Mock }

var _ postgres.APIKeyRepositoryInterface = (*mockAPIKeyRepo)(nil)

func (m *mockAPIKeyRepo) Create(ctx context.Context, tenantID uuid.UUID, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, tenantID, keyHash)
	k, _ := args.Get(0).(*domain.APIKey)
	return k, args.Error(1)
}

func (m *mockAPIKeyRepo) GetTenantByKeyHash(ctx context.Context, keyHash string) (*domain.Tenant, error) {
	args := m.Called(ctx, keyHash)
	t, _ := args.Get(0).(*domain.Tenant)
	return t, args.Error(1)
}

func (m *mockAPIKeyRepo) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, tenantID)
	k, _ := args.Get(0).([]*domain.APIKey)
	return k, args.Error(1)
}

func (m *mockAPIKeyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func newAuthEchoCtx(apiKeyHeader string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if apiKeyHeader != "" {
		req.Header.Set("X-API-Key", apiKeyHeader)
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestAuth_MissingHeader(t *testing.T) {
	repo := &mockAPIKeyRepo{}
	mw := Auth(repo, testLogger(t))

	c, rec := newAuthEchoCtx("")
	called := false
	handler := mw(func(c echo.Context) error {
		called = true
		return nil
	})

	err := handler(c)
	require.NoError(t, err)
	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	repo.AssertNotCalled(t, "GetTenantByKeyHash", mock.Anything, mock.Anything)
}

func TestAuth_InvalidKey(t *testing.T) {
	repo := &mockAPIKeyRepo{}
	repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("badkey")).Return(nil, postgres.ErrNotFound)
	mw := Auth(repo, testLogger(t))

	c, rec := newAuthEchoCtx("badkey")
	called := false
	handler := mw(func(c echo.Context) error {
		called = true
		return nil
	})

	err := handler(c)
	require.NoError(t, err)
	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	repo.AssertExpectations(t)
}

func TestAuth_DBError(t *testing.T) {
	repo := &mockAPIKeyRepo{}
	repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("key")).Return(nil, errors.New("connection refused"))
	mw := Auth(repo, testLogger(t))

	c, rec := newAuthEchoCtx("key")
	called := false
	handler := mw(func(c echo.Context) error {
		called = true
		return nil
	})

	err := handler(c)
	require.NoError(t, err)
	require.False(t, called)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAuth_ValidKey_SetsTenantAndCallsNext(t *testing.T) {
	repo := &mockAPIKeyRepo{}
	tenant := &domain.Tenant{ID: uuid.New(), Name: "Acme"}
	repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("goodkey")).Return(tenant, nil)
	mw := Auth(repo, testLogger(t))

	c, rec := newAuthEchoCtx("goodkey")
	var gotTenant *domain.Tenant
	handler := mw(func(c echo.Context) error {
		gotTenant, _ = c.Get("tenant").(*domain.Tenant)
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotTenant)
	require.Equal(t, tenant.ID, gotTenant.ID)
	repo.AssertExpectations(t)
}
