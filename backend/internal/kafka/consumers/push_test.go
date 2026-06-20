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

// mockPushSender mocks push.Sender.
type mockPushSender struct {
	mock.Mock
}

func (m *mockPushSender) Send(ctx context.Context, token, title, body string) (string, error) {
	args := m.Called(ctx, token, title, body)
	return args.String(0), args.Error(1)
}

func pushHandler(t *testing.T, client interface {
	Send(ctx context.Context, token, title, body string) (string, error)
}, deliveries postgres.NotificationDeliveryRepositoryInterface) func(ctx context.Context, msg *kafkatypes.Message) error {
	t.Helper()
	c, err := NewPushConsumer(Config{BootstrapServers: "localhost:9092"}, nil, client, deliveries, testLogger(t))
	if err != nil {
		t.Fatalf("NewPushConsumer() error = %v", err)
	}
	t.Cleanup(c.Close)
	return c.handler
}

func TestPushHandler_Success_MarksDelivered(t *testing.T) {
	client := &mockPushSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := pushHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientToken: "device-token",
		Subject:        "hi",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientToken, msg.Subject, msg.Body).Return("projects/x/messages/y", nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil", err)
	}

	client.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestPushHandler_MissingToken_ReturnsErrorWithoutSending(t *testing.T) {
	client := &mockPushSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := pushHandler(t, client, deliveries)

	msg := &kafkatypes.Message{NotificationID: uuid.New(), DeliveryID: uuid.New()}

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	client.AssertNotCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestPushHandler_ClientError_ReturnsErrorForRetry(t *testing.T) {
	client := &mockPushSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := pushHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientToken: "device-token",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientToken, msg.Subject, msg.Body).Return("", assertErr)

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	deliveries.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestPushHandler_DeliveryUpdateFails_StillReturnsNil(t *testing.T) {
	client := &mockPushSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := pushHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientToken: "device-token",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientToken, msg.Subject, msg.Body).Return("name", nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(assertErr)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil (delivery update failure is logged, not propagated)", err)
	}
}
