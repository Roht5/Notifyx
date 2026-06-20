package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	"github.com/stretchr/testify/mock"
)

var assertErr = errors.New("boom")

func testLogger(t *testing.T) *logger.Logger {
	log, err := logger.New("test", "error")
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	return log
}

// mockScheduledRepo mocks postgres.ScheduledNotificationRepositoryInterface.
type mockScheduledRepo struct {
	mock.Mock
}

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

var _ postgres.ScheduledNotificationRepositoryInterface = (*mockScheduledRepo)(nil)

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

func newTestScheduler(t *testing.T) (*Scheduler, *mockScheduledRepo, *mockNotificationRepo, *mockDeliveryRepo, *mockProducer) {
	scheduled := &mockScheduledRepo{}
	notifications := &mockNotificationRepo{}
	deliveries := &mockDeliveryRepo{}
	prod := &mockProducer{}
	s := New(scheduled, notifications, deliveries, prod, testLogger(t))
	return s, scheduled, notifications, deliveries, prod
}

func sampleScheduledNotification(notificationID uuid.UUID) *domain.ScheduledNotification {
	return &domain.ScheduledNotification{
		ID:             uuid.New(),
		NotificationID: notificationID,
		ScheduledAt:    time.Now().Add(-time.Minute),
	}
}

func sampleSchedNotificationRow(id uuid.UUID) *domain.Notification {
	return &domain.Notification{
		ID:             id,
		TenantID:       uuid.New(),
		Channel:        domain.ChannelEmail,
		Priority:       domain.PriorityNormal,
		Status:         domain.StatusQueued,
		RecipientEmail: "user@example.com",
		Body:           "hello",
	}
}

func TestFireOne_DueAndUnfired_PublishesAndMarksFired(t *testing.T) {
	s, scheduled, notifications, deliveries, prod := newTestScheduler(t)
	ctx := context.Background()

	notificationID := uuid.New()
	sn := sampleScheduledNotification(notificationID)
	n := sampleSchedNotificationRow(notificationID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusPending}

	notifications.On("GetByID", mock.Anything, notificationID).Return(n, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, kafkatypes.TopicForChannel(n.Channel), mock.Anything).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, n.ID, domain.StatusQueued).Return(nil)
	deliveries.On("SetStatus", mock.Anything, delivery.ID, domain.StatusQueued).Return(nil)
	scheduled.On("MarkFired", mock.Anything, sn.ID).Return(nil)

	s.fireOne(ctx, sn)

	notifications.AssertExpectations(t)
	deliveries.AssertExpectations(t)
	prod.AssertExpectations(t)
	scheduled.AssertExpectations(t)
}

func TestTickOnce_NoDueNotifications_SkipsFireAndDoesNotPublish(t *testing.T) {
	s, scheduled, _, _, prod := newTestScheduler(t)
	ctx := context.Background()

	scheduled.On("GetDueUnfired", mock.Anything, mock.Anything, batchSize).Return([]*domain.ScheduledNotification{}, nil)

	s.tickOnce(ctx)

	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
	scheduled.AssertNotCalled(t, "MarkFired", mock.Anything, mock.Anything)
}

func TestFireOne_PublishFailure_DoesNotMarkFired(t *testing.T) {
	s, scheduled, notifications, deliveries, prod := newTestScheduler(t)
	ctx := context.Background()

	notificationID := uuid.New()
	sn := sampleScheduledNotification(notificationID)
	n := sampleSchedNotificationRow(notificationID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusPending}

	notifications.On("GetByID", mock.Anything, notificationID).Return(n, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(assertErr)

	s.fireOne(ctx, sn)

	scheduled.AssertNotCalled(t, "MarkFired", mock.Anything, mock.Anything)
	notifications.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
	deliveries.AssertNotCalled(t, "SetStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestFireOne_NilProducer_LeavesUnfired(t *testing.T) {
	scheduled := &mockScheduledRepo{}
	notifications := &mockNotificationRepo{}
	deliveries := &mockDeliveryRepo{}
	s := New(scheduled, notifications, deliveries, nil, testLogger(t))
	ctx := context.Background()

	notificationID := uuid.New()
	sn := sampleScheduledNotification(notificationID)
	n := sampleSchedNotificationRow(notificationID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusPending}

	notifications.On("GetByID", mock.Anything, notificationID).Return(n, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)

	s.fireOne(ctx, sn)

	scheduled.AssertNotCalled(t, "MarkFired", mock.Anything, mock.Anything)
	notifications.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestFireOne_GetByIDError_DoesNotPublish(t *testing.T) {
	s, scheduled, notifications, deliveries, prod := newTestScheduler(t)
	ctx := context.Background()

	notificationID := uuid.New()
	sn := sampleScheduledNotification(notificationID)

	notifications.On("GetByID", mock.Anything, notificationID).Return(nil, assertErr)

	s.fireOne(ctx, sn)

	deliveries.AssertNotCalled(t, "GetByNotificationID", mock.Anything, mock.Anything)
	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
	scheduled.AssertNotCalled(t, "MarkFired", mock.Anything, mock.Anything)
}

func TestFireOne_MissingDeliveryRows_DoesNotPublish(t *testing.T) {
	s, scheduled, notifications, deliveries, prod := newTestScheduler(t)
	ctx := context.Background()

	notificationID := uuid.New()
	sn := sampleScheduledNotification(notificationID)
	n := sampleSchedNotificationRow(notificationID)

	notifications.On("GetByID", mock.Anything, notificationID).Return(n, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return(nil, nil)

	s.fireOne(ctx, sn)

	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
	scheduled.AssertNotCalled(t, "MarkFired", mock.Anything, mock.Anything)
}

func TestFireOne_MarkFiredError_StillCompletesWithoutPanicking(t *testing.T) {
	s, scheduled, notifications, deliveries, prod := newTestScheduler(t)
	ctx := context.Background()

	notificationID := uuid.New()
	sn := sampleScheduledNotification(notificationID)
	n := sampleSchedNotificationRow(notificationID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusPending}

	notifications.On("GetByID", mock.Anything, notificationID).Return(n, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, n.ID, domain.StatusQueued).Return(nil)
	deliveries.On("SetStatus", mock.Anything, delivery.ID, domain.StatusQueued).Return(nil)
	scheduled.On("MarkFired", mock.Anything, sn.ID).Return(assertErr)

	s.fireOne(ctx, sn)

	scheduled.AssertExpectations(t)
}

func TestTickOnce_GetDueUnfiredError_LogsAndReturns(t *testing.T) {
	s, scheduled, _, _, prod := newTestScheduler(t)
	ctx := context.Background()

	scheduled.On("GetDueUnfired", mock.Anything, mock.Anything, batchSize).Return(nil, assertErr)

	s.tickOnce(ctx)

	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestTickOnce_DueNotification_FiresAndMarksFired(t *testing.T) {
	s, scheduled, notifications, deliveries, prod := newTestScheduler(t)
	ctx := context.Background()

	notificationID := uuid.New()
	sn := sampleScheduledNotification(notificationID)
	n := sampleSchedNotificationRow(notificationID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusPending}

	scheduled.On("GetDueUnfired", mock.Anything, mock.Anything, batchSize).Return([]*domain.ScheduledNotification{sn}, nil)
	notifications.On("GetByID", mock.Anything, notificationID).Return(n, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, kafkatypes.TopicForChannel(n.Channel), mock.Anything).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, n.ID, domain.StatusQueued).Return(nil)
	deliveries.On("SetStatus", mock.Anything, delivery.ID, domain.StatusQueued).Return(nil)
	scheduled.On("MarkFired", mock.Anything, sn.ID).Return(nil)

	s.tickOnce(ctx)

	scheduled.AssertExpectations(t)
	prod.AssertExpectations(t)
}

func TestRun_TicksAndStopsOnContextCancel(t *testing.T) {
	s, scheduled, _, _, _ := newTestScheduler(t)
	scheduled.On("GetDueUnfired", mock.Anything, mock.Anything, batchSize).Return([]*domain.ScheduledNotification{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx, 5*time.Millisecond)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
