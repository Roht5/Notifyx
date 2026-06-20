package consumers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/stretchr/testify/mock"
)

// mockEmailSender mocks email.Sender.
type mockEmailSender struct {
	mock.Mock
}

func (m *mockEmailSender) Send(ctx context.Context, to, subject, body string) (string, error) {
	args := m.Called(ctx, to, subject, body)
	return args.String(0), args.Error(1)
}

// mockDeliveryRepoChannel mocks postgres.NotificationDeliveryRepositoryInterface for the
// email/push/sms channel consumer tests.
type mockDeliveryRepoChannel struct {
	mock.Mock
}

func (m *mockDeliveryRepoChannel) Create(ctx context.Context, notificationID uuid.UUID, channel domain.Channel, status domain.Status) (*domain.NotificationDelivery, error) {
	args := m.Called(ctx, notificationID, channel, status)
	d, _ := args.Get(0).(*domain.NotificationDelivery)
	return d, args.Error(1)
}

func (m *mockDeliveryRepoChannel) SetStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockDeliveryRepoChannel) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, errorMessage string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *mockDeliveryRepoChannel) GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*domain.NotificationDelivery, error) {
	args := m.Called(ctx, notificationID)
	d, _ := args.Get(0).([]*domain.NotificationDelivery)
	return d, args.Error(1)
}

var _ postgres.NotificationDeliveryRepositoryInterface = (*mockDeliveryRepoChannel)(nil)

func emailHandler(t *testing.T, client interface {
	Send(ctx context.Context, to, subject, body string) (string, error)
}, deliveries postgres.NotificationDeliveryRepositoryInterface) func(ctx context.Context, msg *kafkatypes.Message) error {
	t.Helper()
	c, err := NewEmailConsumer(Config{BootstrapServers: "localhost:9092"}, nil, client, deliveries, testLogger(t))
	if err != nil {
		t.Fatalf("NewEmailConsumer() error = %v", err)
	}
	t.Cleanup(c.Close)
	return c.handler
}

func TestEmailHandler_Success_MarksDelivered(t *testing.T) {
	client := &mockEmailSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := emailHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientEmail: "user@example.com",
		Subject:        "hi",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientEmail, msg.Subject, msg.Body).Return("msg_123", nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil", err)
	}

	client.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestEmailHandler_MissingRecipient_ReturnsErrorWithoutSending(t *testing.T) {
	client := &mockEmailSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := emailHandler(t, client, deliveries)

	msg := &kafkatypes.Message{NotificationID: uuid.New(), DeliveryID: uuid.New()}

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	client.AssertNotCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestEmailHandler_ClientError_MarksFailedPath_ReturnsErrorForRetry(t *testing.T) {
	client := &mockEmailSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := emailHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientEmail: "user@example.com",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientEmail, msg.Subject, msg.Body).Return("", assertErr)

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	deliveries.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestEmailHandler_DeliveryUpdateFails_StillReturnsNil(t *testing.T) {
	client := &mockEmailSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := emailHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientEmail: "user@example.com",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientEmail, msg.Subject, msg.Body).Return("msg_123", nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(assertErr)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil (delivery update failure is logged, not propagated)", err)
	}
}
