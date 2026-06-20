package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	"github.com/stretchr/testify/mock"
)

// --- test helpers ---

func newTestLogger() *logger.Logger {
	l, err := logger.New("test", "error")
	if err != nil {
		panic(err)
	}
	return l
}

// newCtx builds an echo.Context the way handlers expect to receive it: a JSON request
// body (or none), a recorder to inspect the response, and optionally a tenant injected
// into context (mirroring what the auth middleware does in production).
func newCtx(method, target, body string, tenant *domain.Tenant) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if tenant != nil {
		c.Set("tenant", tenant)
	}
	return c, rec
}

func testTenant() *domain.Tenant {
	return &domain.Tenant{
		ID:            uuid.New(),
		Name:          "Acme",
		GlobalRateCap: 1000,
		CreatedAt:     time.Now(),
	}
}

// --- mock repositories ---

type mockNotificationRepo struct{ mock.Mock }

var _ postgres.NotificationRepositoryInterface = (*mockNotificationRepo)(nil)

func (m *mockNotificationRepo) Create(ctx context.Context, p postgres.CreateNotificationParams) (*domain.Notification, error) {
	args := m.Called(ctx, p)
	n, _ := args.Get(0).(*domain.Notification)
	return n, args.Error(1)
}

func (m *mockNotificationRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	n, _ := args.Get(0).(*domain.Notification)
	return n, args.Error(1)
}

func (m *mockNotificationRepo) GetByTenantID(ctx context.Context, tenantID uuid.UUID, f postgres.NotificationFilter) ([]*domain.Notification, int, error) {
	args := m.Called(ctx, tenantID, f)
	n, _ := args.Get(0).([]*domain.Notification)
	return n, args.Int(1), args.Error(2)
}

func (m *mockNotificationRepo) GetByStatus(ctx context.Context, status domain.Status, limit int) ([]*domain.Notification, error) {
	args := m.Called(ctx, status, limit)
	n, _ := args.Get(0).([]*domain.Notification)
	return n, args.Error(1)
}

func (m *mockNotificationRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockNotificationRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	args := m.Called(ctx, now)
	return args.Get(0).(int64), args.Error(1)
}

type mockDeliveryRepo struct{ mock.Mock }

var _ postgres.NotificationDeliveryRepositoryInterface = (*mockDeliveryRepo)(nil)

func (m *mockDeliveryRepo) Create(ctx context.Context, notificationID uuid.UUID, channel domain.Channel, status domain.Status) (*domain.NotificationDelivery, error) {
	args := m.Called(ctx, notificationID, channel, status)
	d, _ := args.Get(0).(*domain.NotificationDelivery)
	return d, args.Error(1)
}

func (m *mockDeliveryRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockDeliveryRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, errorMessage string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *mockDeliveryRepo) GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*domain.NotificationDelivery, error) {
	args := m.Called(ctx, notificationID)
	d, _ := args.Get(0).([]*domain.NotificationDelivery)
	return d, args.Error(1)
}

type mockChannelRepo struct{ mock.Mock }

var _ postgres.TenantChannelRepositoryInterface = (*mockChannelRepo)(nil)

func (m *mockChannelRepo) Upsert(ctx context.Context, tenantID uuid.UUID, channel domain.Channel, enabled bool) error {
	args := m.Called(ctx, tenantID, channel, enabled)
	return args.Error(0)
}

func (m *mockChannelRepo) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantChannel, error) {
	args := m.Called(ctx, tenantID)
	c, _ := args.Get(0).([]*domain.TenantChannel)
	return c, args.Error(1)
}

func (m *mockChannelRepo) IsEnabled(ctx context.Context, tenantID uuid.UUID, channel domain.Channel) (bool, error) {
	args := m.Called(ctx, tenantID, channel)
	return args.Bool(0), args.Error(1)
}

type mockTemplateRepo struct{ mock.Mock }

var _ postgres.TemplateRepositoryInterface = (*mockTemplateRepo)(nil)

func (m *mockTemplateRepo) Create(ctx context.Context, tenantID uuid.UUID, name string, channel domain.Channel, subject, body string) (*domain.NotificationTemplate, error) {
	args := m.Called(ctx, tenantID, name, channel, subject, body)
	t, _ := args.Get(0).(*domain.NotificationTemplate)
	return t, args.Error(1)
}

func (m *mockTemplateRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.NotificationTemplate, error) {
	args := m.Called(ctx, tenantID, id)
	t, _ := args.Get(0).(*domain.NotificationTemplate)
	return t, args.Error(1)
}

func (m *mockTemplateRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.NotificationTemplate, error) {
	args := m.Called(ctx, tenantID)
	t, _ := args.Get(0).([]*domain.NotificationTemplate)
	return t, args.Error(1)
}

func (m *mockTemplateRepo) Update(ctx context.Context, tenantID, id uuid.UUID, name string, channel domain.Channel, subject, body string) (*domain.NotificationTemplate, error) {
	args := m.Called(ctx, tenantID, id, name, channel, subject, body)
	t, _ := args.Get(0).(*domain.NotificationTemplate)
	return t, args.Error(1)
}

func (m *mockTemplateRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	args := m.Called(ctx, tenantID, id)
	return args.Error(0)
}

type mockScheduledRepo struct{ mock.Mock }

var _ postgres.ScheduledNotificationRepositoryInterface = (*mockScheduledRepo)(nil)

func (m *mockScheduledRepo) Create(ctx context.Context, notificationID uuid.UUID, scheduledAt time.Time) (*domain.ScheduledNotification, error) {
	args := m.Called(ctx, notificationID, scheduledAt)
	s, _ := args.Get(0).(*domain.ScheduledNotification)
	return s, args.Error(1)
}

func (m *mockScheduledRepo) GetDueUnfired(ctx context.Context, now time.Time, limit int) ([]*domain.ScheduledNotification, error) {
	args := m.Called(ctx, now, limit)
	s, _ := args.Get(0).([]*domain.ScheduledNotification)
	return s, args.Error(1)
}

func (m *mockScheduledRepo) MarkFired(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockTenantRepo struct{ mock.Mock }

var _ postgres.TenantRepositoryInterface = (*mockTenantRepo)(nil)

func (m *mockTenantRepo) Create(ctx context.Context, name string, globalRateCap int) (*domain.Tenant, error) {
	args := m.Called(ctx, name, globalRateCap)
	t, _ := args.Get(0).(*domain.Tenant)
	return t, args.Error(1)
}

func (m *mockTenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	args := m.Called(ctx, id)
	t, _ := args.Get(0).(*domain.Tenant)
	return t, args.Error(1)
}

func (m *mockTenantRepo) GetAll(ctx context.Context) ([]*domain.Tenant, error) {
	args := m.Called(ctx)
	t, _ := args.Get(0).([]*domain.Tenant)
	return t, args.Error(1)
}

func (m *mockTenantRepo) Update(ctx context.Context, id uuid.UUID, name string) (*domain.Tenant, error) {
	args := m.Called(ctx, id, name)
	t, _ := args.Get(0).(*domain.Tenant)
	return t, args.Error(1)
}

func (m *mockTenantRepo) UpdateGlobalRateCap(ctx context.Context, id uuid.UUID, globalRateCap int) (*domain.Tenant, error) {
	args := m.Called(ctx, id, globalRateCap)
	t, _ := args.Get(0).(*domain.Tenant)
	return t, args.Error(1)
}

func (m *mockTenantRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
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

type mockRateLimitRepo struct{ mock.Mock }

var _ postgres.TenantRateLimitRepositoryInterface = (*mockRateLimitRepo)(nil)

func (m *mockRateLimitRepo) Upsert(ctx context.Context, tenantID uuid.UUID, channel domain.Channel, maxPerMin int) error {
	args := m.Called(ctx, tenantID, channel, maxPerMin)
	return args.Error(0)
}

func (m *mockRateLimitRepo) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantRateLimit, error) {
	args := m.Called(ctx, tenantID)
	r, _ := args.Get(0).([]*domain.TenantRateLimit)
	return r, args.Error(1)
}

func (m *mockRateLimitRepo) GetByChannel(ctx context.Context, tenantID uuid.UUID, channel domain.Channel) (*domain.TenantRateLimit, error) {
	args := m.Called(ctx, tenantID, channel)
	r, _ := args.Get(0).(*domain.TenantRateLimit)
	return r, args.Error(1)
}

type mockAnalyticsRepo struct{ mock.Mock }

var _ postgres.AnalyticsRepositoryInterface = (*mockAnalyticsRepo)(nil)

func (m *mockAnalyticsRepo) Summary(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (*domain.AnalyticsSummary, error) {
	args := m.Called(ctx, tenantID, from, to)
	s, _ := args.Get(0).(*domain.AnalyticsSummary)
	return s, args.Error(1)
}

func (m *mockAnalyticsRepo) ChannelBreakdown(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*domain.ChannelBreakdown, error) {
	args := m.Called(ctx, tenantID, from, to)
	c, _ := args.Get(0).([]*domain.ChannelBreakdown)
	return c, args.Error(1)
}

func (m *mockAnalyticsRepo) DLQTrend(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*domain.DLQTrendPoint, error) {
	args := m.Called(ctx, tenantID, from, to)
	d, _ := args.Get(0).([]*domain.DLQTrendPoint)
	return d, args.Error(1)
}

// --- mock producer / rate limiter / dedup ---

type mockProducer struct{ mock.Mock }

func (m *mockProducer) Publish(ctx context.Context, topic string, msg *kafkatypes.Message) error {
	args := m.Called(ctx, topic, msg)
	return args.Error(0)
}

type mockRateLimiter struct{ mock.Mock }

func (m *mockRateLimiter) Allow(ctx context.Context, tenantID uuid.UUID, channel domain.Channel) (bool, error) {
	args := m.Called(ctx, tenantID, channel)
	return args.Bool(0), args.Error(1)
}

type mockDedup struct{ mock.Mock }

func (m *mockDedup) Reserve(ctx context.Context, idempotencyKey string, notificationID uuid.UUID) (bool, uuid.UUID, error) {
	args := m.Called(ctx, idempotencyKey, notificationID)
	id, _ := args.Get(1).(uuid.UUID)
	return args.Bool(0), id, args.Error(2)
}

func (m *mockDedup) Release(ctx context.Context, idempotencyKey string) error {
	args := m.Called(ctx, idempotencyKey)
	return args.Error(0)
}
