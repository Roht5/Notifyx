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

// mockSMSSender mocks sms.Sender.
type mockSMSSender struct {
	mock.Mock
}

func (m *mockSMSSender) Send(ctx context.Context, phone, message string) (string, error) {
	args := m.Called(ctx, phone, message)
	return args.String(0), args.Error(1)
}

func smsHandler(t *testing.T, client interface {
	Send(ctx context.Context, phone, message string) (string, error)
}, deliveries postgres.NotificationDeliveryRepositoryInterface) func(ctx context.Context, msg *kafkatypes.Message) error {
	t.Helper()
	c, err := NewSMSConsumer(Config{BootstrapServers: "localhost:9092"}, nil, client, deliveries, testLogger(t))
	if err != nil {
		t.Fatalf("NewSMSConsumer() error = %v", err)
	}
	t.Cleanup(c.Close)
	return c.handler
}

func TestSMSHandler_Success_MarksDelivered(t *testing.T) {
	client := &mockSMSSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := smsHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientPhone: "9999999999",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientPhone, msg.Body).Return("req_123", nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(nil)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil", err)
	}

	client.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestSMSHandler_MissingPhone_ReturnsErrorWithoutSending(t *testing.T) {
	client := &mockSMSSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := smsHandler(t, client, deliveries)

	msg := &kafkatypes.Message{NotificationID: uuid.New(), DeliveryID: uuid.New()}

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	client.AssertNotCalled(t, "Send", mock.Anything, mock.Anything, mock.Anything)
}

func TestSMSHandler_ClientError_ReturnsErrorForRetry(t *testing.T) {
	client := &mockSMSSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := smsHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientPhone: "9999999999",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientPhone, msg.Body).Return("", assertErr)

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("handler() error = nil, want non-nil")
	}
	deliveries.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSMSHandler_DeliveryUpdateFails_StillReturnsNil(t *testing.T) {
	client := &mockSMSSender{}
	deliveries := &mockDeliveryRepoChannel{}
	handler := smsHandler(t, client, deliveries)

	msg := &kafkatypes.Message{
		NotificationID: uuid.New(),
		DeliveryID:     uuid.New(),
		RecipientPhone: "9999999999",
		Body:           "hello",
	}

	client.On("Send", mock.Anything, msg.RecipientPhone, msg.Body).Return("req_123", nil)
	deliveries.On("UpdateStatus", mock.Anything, msg.DeliveryID, domain.StatusDelivered, "").Return(assertErr)

	err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler() error = %v, want nil (delivery update failure is logged, not propagated)", err)
	}
}
