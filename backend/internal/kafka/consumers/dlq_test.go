package consumers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/stretchr/testify/mock"
)

// mockDLQRepo mocks postgres.DLQRepositoryInterface.
type mockDLQRepo struct {
	mock.Mock
}

func (m *mockDLQRepo) Create(ctx context.Context, msg *kafkatypes.Message, errMsg string) error {
	args := m.Called(ctx, msg, errMsg)
	return args.Error(0)
}

var _ postgres.DLQRepositoryInterface = (*mockDLQRepo)(nil)

// mockNotificationRepoDLQ mocks postgres.NotificationRepositoryInterface for dlq_test.go.
type mockNotificationRepoDLQ struct {
	mock.Mock
}

func (m *mockNotificationRepoDLQ) Create(ctx context.Context, p postgres.CreateNotificationParams) (*domain.Notification, error) {
	args := m.Called(ctx, p)
	n, _ := args.Get(0).(*domain.Notification)
	return n, args.Error(1)
}

func (m *mockNotificationRepoDLQ) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	n, _ := args.Get(0).(*domain.Notification)
	return n, args.Error(1)
}

func (m *mockNotificationRepoDLQ) GetByTenantID(ctx context.Context, tenantID uuid.UUID, f postgres.NotificationFilter) ([]*domain.Notification, int, error) {
	args := m.Called(ctx, tenantID, f)
	n, _ := args.Get(0).([]*domain.Notification)
	return n, args.Int(1), args.Error(2)
}

func (m *mockNotificationRepoDLQ) GetByStatus(ctx context.Context, status domain.Status, limit int) ([]*domain.Notification, error) {
	args := m.Called(ctx, status, limit)
	n, _ := args.Get(0).([]*domain.Notification)
	return n, args.Error(1)
}

func (m *mockNotificationRepoDLQ) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockNotificationRepoDLQ) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	args := m.Called(ctx, now)
	return args.Get(0).(int64), args.Error(1)
}

var _ postgres.NotificationRepositoryInterface = (*mockNotificationRepoDLQ)(nil)

// mockDeliveryRepoDLQ mocks postgres.NotificationDeliveryRepositoryInterface for dlq_test.go.
type mockDeliveryRepoDLQ struct {
	mock.Mock
}

func (m *mockDeliveryRepoDLQ) Create(ctx context.Context, notificationID uuid.UUID, channel domain.Channel, status domain.Status) (*domain.NotificationDelivery, error) {
	args := m.Called(ctx, notificationID, channel, status)
	d, _ := args.Get(0).(*domain.NotificationDelivery)
	return d, args.Error(1)
}

func (m *mockDeliveryRepoDLQ) SetStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockDeliveryRepoDLQ) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, errorMessage string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *mockDeliveryRepoDLQ) GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*domain.NotificationDelivery, error) {
	args := m.Called(ctx, notificationID)
	d, _ := args.Get(0).([]*domain.NotificationDelivery)
	return d, args.Error(1)
}

var _ postgres.NotificationDeliveryRepositoryInterface = (*mockDeliveryRepoDLQ)(nil)

// dlqHandler builds a Consumer via NewDLQConsumer and returns its handler so tests can
// invoke the HandlerFunc directly without going through Kafka.
func dlqHandler(t *testing.T, dlqRepo postgres.DLQRepositoryInterface, notifications postgres.NotificationRepositoryInterface, deliveries postgres.NotificationDeliveryRepositoryInterface) func(ctx context.Context, msg *kafkatypes.Message) error {
	t.Helper()
	c, err := NewDLQConsumer(Config{BootstrapServers: "localhost:9092"}, dlqRepo, notifications, deliveries, testLogger(t))
	if err != nil {
		t.Fatalf("NewDLQConsumer() error = %v", err)
	}
	t.Cleanup(c.Close)
	return c.handler
}

func TestDLQHandler_PersistsAndMarksFailed(t *testing.T) {
	dlqRepo := &mockDLQRepo{}
	notifications := &mockNotificationRepoDLQ{}
	deliveries := &mockDeliveryRepoDLQ{}
	handler := dlqHandler(t, dlqRepo, notifications, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		Channel:        domain.ChannelEmail,
		LastError:      "resend send failed: boom",
	}

	dlqRepo.On("Create", mock.Anything, msg, msg.LastError).Return(nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusFailed, msg.LastError).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, msg.NotificationID, domain.StatusFailed).Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil", err)
	}

	dlqRepo.AssertExpectations(t)
	deliveries.AssertExpectations(t)
	notifications.AssertExpectations(t)
}

func TestDLQHandler_NoLastError_UsesDefaultReason(t *testing.T) {
	dlqRepo := &mockDLQRepo{}
	notifications := &mockNotificationRepoDLQ{}
	deliveries := &mockDeliveryRepoDLQ{}
	handler := dlqHandler(t, dlqRepo, notifications, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		Channel:        domain.ChannelSMS,
	}
	wantReason := "exhausted delivery retries (no error recorded)"

	dlqRepo.On("Create", mock.Anything, msg, wantReason).Return(nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusFailed, wantReason).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, msg.NotificationID, domain.StatusFailed).Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil", err)
	}

	dlqRepo.AssertExpectations(t)
}

func TestDLQHandler_CreateFails_ReturnsErrorAndSkipsStatusUpdates(t *testing.T) {
	dlqRepo := &mockDLQRepo{}
	notifications := &mockNotificationRepoDLQ{}
	deliveries := &mockDeliveryRepoDLQ{}
	handler := dlqHandler(t, dlqRepo, notifications, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		Channel:        domain.ChannelPush,
		LastError:      "fcm send failed",
	}

	dlqRepo.On("Create", mock.Anything, msg, msg.LastError).Return(assertErr)

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}

	deliveries.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	notifications.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestDLQHandler_DeliveryUpdateFails_StillMarksNotificationFailedAndReturnsNil(t *testing.T) {
	dlqRepo := &mockDLQRepo{}
	notifications := &mockNotificationRepoDLQ{}
	deliveries := &mockDeliveryRepoDLQ{}
	handler := dlqHandler(t, dlqRepo, notifications, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		Channel:        domain.ChannelEmail,
		LastError:      "boom",
	}

	dlqRepo.On("Create", mock.Anything, msg, msg.LastError).Return(nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusFailed, msg.LastError).Return(assertErr)
	notifications.On("UpdateStatus", mock.Anything, msg.NotificationID, domain.StatusFailed).Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil (delivery update failure is logged, not propagated)", err)
	}

	notifications.AssertExpectations(t)
}
