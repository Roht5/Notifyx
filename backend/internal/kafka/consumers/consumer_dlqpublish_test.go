package consumers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
)

// mockProducerDLQ mocks producer.ProducerInterface for exercising handleWithRetry's
// retry-exhaustion-then-DLQ-publish branches, which the rest of this package's
// consumer_test.go only covers via the producer==nil ("drop") path.
type mockProducerDLQ struct {
	mock.Mock
}

func (m *mockProducerDLQ) Publish(ctx context.Context, topic string, msg *kafkatypes.Message) error {
	args := m.Called(ctx, topic, msg)
	return args.Error(0)
}

// withShrunkBackoff temporarily replaces the package-level retryBackoff table with
// millisecond-scale durations so retry-exhaustion tests don't pay the real 1s+2s+4s cost,
// restoring the original on cleanup.
func withShrunkBackoff(t *testing.T) {
	t.Helper()
	original := retryBackoff
	retryBackoff = []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}
	t.Cleanup(func() { retryBackoff = original })
}

// TestHandleWithRetry_DLQPublishSucceeds_AfterExhaustion verifies that once retries are
// exhausted and a producer IS configured (the normal, non-DLQ consumer case), the message
// is published to the DLQ topic and handleWithRetry returns true (safe to commit).
func TestHandleWithRetry_DLQPublishSucceeds_AfterExhaustion(t *testing.T) {
	withShrunkBackoff(t)
	notificationID := uuid.New()
	prod := &mockProducerDLQ{}
	prod.On("Publish", mock.Anything, kafkatypes.TopicDLQ, mock.MatchedBy(func(msg *kafkatypes.Message) bool {
		return msg.NotificationID == notificationID && msg.LastError != ""
	})).Return(nil)

	c := &Consumer{
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			return assertErr
		},
		log:      testLogger(t),
		topic:    "test-topic",
		producer: prod,
	}

	km := kafka.Message{Value: validMessageBytes(t, notificationID)}
	ok := c.handleWithRetry(context.Background(), km)

	assert.True(t, ok, "successful DLQ publish after exhaustion must be safe to commit")
	prod.AssertExpectations(t)
}

// TestHandleWithRetry_DLQPublishFails_AfterExhaustion verifies that if the DLQ publish
// itself fails, handleWithRetry returns false so the offset is NOT committed and the
// message is redelivered — losing it here would mean no record anywhere.
func TestHandleWithRetry_DLQPublishFails_AfterExhaustion(t *testing.T) {
	withShrunkBackoff(t)
	notificationID := uuid.New()
	prod := &mockProducerDLQ{}
	prod.On("Publish", mock.Anything, kafkatypes.TopicDLQ, mock.Anything).Return(assertErr)

	c := &Consumer{
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			return assertErr
		},
		log:      testLogger(t),
		topic:    "test-topic",
		producer: prod,
	}

	km := kafka.Message{Value: validMessageBytes(t, notificationID)}
	ok := c.handleWithRetry(context.Background(), km)

	assert.False(t, ok, "failed DLQ publish must not be committed — message would be lost")
	prod.AssertExpectations(t)
}
