package consumers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
)

// mockNotifier mocks ws.Notifier.
type mockNotifier struct {
	mock.Mock
}

func (m *mockNotifier) SendToUser(tenantID, userID string, payload []byte) bool {
	args := m.Called(tenantID, userID, payload)
	return args.Bool(0)
}

// mockPusher mocks offlinequeue.Pusher.
type mockPusher struct {
	mock.Mock
}

func (m *mockPusher) Push(ctx context.Context, tenantID, userID string, payload []byte) error {
	args := m.Called(ctx, tenantID, userID, payload)
	return args.Error(0)
}

func inappHandler(t *testing.T, hub interface {
	SendToUser(tenantID, userID string, payload []byte) bool
}, queue interface {
	Push(ctx context.Context, tenantID, userID string, payload []byte) error
}, deliveries postgres.NotificationDeliveryRepositoryInterface) func(ctx context.Context, msg *kafkatypes.Message) error {
	t.Helper()

	var queueArg interface {
		Push(ctx context.Context, tenantID, userID string, payload []byte) error
	}
	queueArg = queue // may be a nil interface if caller passed nil directly

	c, err := NewInAppConsumer(Config{BootstrapServers: "localhost:9092"}, nil, hub, queueArg, deliveries, testLogger(t))
	if err != nil {
		t.Fatalf("NewInAppConsumer() error = %v", err)
	}
	t.Cleanup(c.Close)
	return c.handler
}

func TestInAppHandler_OnlineDelivery_Succeeds(t *testing.T) {
	hub := &mockNotifier{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := inappHandler(t, hub, nil, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		RecipientID:    "user-1",
		Body:           "hello",
	}

	hub.On("SendToUser", msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(true)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil", err)
	}

	hub.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestInAppHandler_MissingRecipientID_ReturnsErrorWithoutSending(t *testing.T) {
	hub := &mockNotifier{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := inappHandler(t, hub, nil, deliveries)

	msg := &kafkatypes.Message{NotificationID: uuid.New(), DeliveryID: uuid.New(), TenantID: uuid.New()}

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	hub.AssertNotCalled(t, "SendToUser", mock.Anything, mock.Anything, mock.Anything)
}

func TestInAppHandler_OfflineFallsBackToQueue_Succeeds(t *testing.T) {
	hub := &mockNotifier{}
	queue := &mockPusher{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := inappHandler(t, hub, queue, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		RecipientID:    "user-1",
		Body:           "hello",
	}

	hub.On("SendToUser", msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(false)
	queue.On("Push", mock.Anything, msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(nil)
	deliveries.On("SetStatus", mock.Anything, msg.DeliveryID, domain.StatusQueued).Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil", err)
	}

	hub.AssertExpectations(t)
	queue.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestInAppHandler_OfflineNoQueueConfigured_ReturnsError(t *testing.T) {
	hub := &mockNotifier{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := inappHandler(t, hub, nil, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		RecipientID:    "user-1",
		Body:           "hello",
	}

	hub.On("SendToUser", msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(false)

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	deliveries.AssertNotCalled(t, "SetStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestInAppHandler_QueuePushFails_ReturnsError(t *testing.T) {
	hub := &mockNotifier{}
	queue := &mockPusher{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := inappHandler(t, hub, queue, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		RecipientID:    "user-1",
		Body:           "hello",
	}

	hub.On("SendToUser", msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(false)
	queue.On("Push", mock.Anything, msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(assertErr)

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	deliveries.AssertNotCalled(t, "SetStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestInAppHandler_SetStatusQueuedFails_StillReturnsNil(t *testing.T) {
	hub := &mockNotifier{}
	queue := &mockPusher{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := inappHandler(t, hub, queue, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		RecipientID:    "user-1",
		Body:           "hello",
	}

	hub.On("SendToUser", msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(false)
	queue.On("Push", mock.Anything, msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(nil)
	deliveries.On("SetStatus", mock.Anything, msg.DeliveryID, domain.StatusQueued).Return(assertErr)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil (set-status failure is logged, not propagated)", err)
	}
}

func TestInAppHandler_DeliveryUpdateFails_StillReturnsNil(t *testing.T) {
	hub := &mockNotifier{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := inappHandler(t, hub, nil, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		RecipientID:    "user-1",
		Body:           "hello",
	}

	hub.On("SendToUser", msg.TenantID.String(), msg.RecipientID, mock.Anything).Return(true)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(assertErr)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil (delivery update failure is logged, not propagated)", err)
	}
}
