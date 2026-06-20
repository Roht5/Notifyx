package ratelimit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/stretchr/testify/mock"
)

// mockTenantRepo mocks postgres.TenantRepositoryInterface.
type mockTenantRepo struct {
	mock.Mock
}

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

var _ postgres.TenantRepositoryInterface = (*mockTenantRepo)(nil)

// mockRateLimitRepo mocks postgres.TenantRateLimitRepositoryInterface.
type mockRateLimitRepo struct {
	mock.Mock
}

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

var _ postgres.TenantRateLimitRepositoryInterface = (*mockRateLimitRepo)(nil)

// mockNotificationRepo mocks postgres.NotificationRepositoryInterface.
type mockNotificationRepo struct {
	mock.Mock
}

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

var _ postgres.NotificationRepositoryInterface = (*mockNotificationRepo)(nil)

// mockDeliveryRepo mocks postgres.NotificationDeliveryRepositoryInterface.
type mockDeliveryRepo struct {
	mock.Mock
}

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

var _ postgres.NotificationDeliveryRepositoryInterface = (*mockDeliveryRepo)(nil)

// mockProducer mocks producer.ProducerInterface.
type mockProducer struct {
	mock.Mock
}

func (m *mockProducer) Publish(ctx context.Context, topic string, msg *kafkatypes.Message) error {
	args := m.Called(ctx, topic, msg)
	return args.Error(0)
}
