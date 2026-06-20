package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// assertErr is a shared sentinel error used across this package's tests to simulate
// failures from mocked dependencies.
var assertErr = errors.New("boom")

// testLogger returns a usable *logger.Logger backed by zap's no-op-ish development
// config — cheap enough to build per-test and avoids needing a separate constructor seam.
func testLogger(t *testing.T) *logger.Logger {
	t.Helper()
	l, err := logger.New("test", "error") // error level keeps test output quiet
	require.NoError(t, err)
	return l
}

func validMessageBytes(t *testing.T, notificationID uuid.UUID) []byte {
	t.Helper()
	msg := kafkatypes.Message{
		NotificationID: notificationID,
		DeliveryID:     uuid.New(),
		TenantID:       uuid.New(),
		Channel:        domain.ChannelEmail,
		Priority:       domain.PriorityHigh,
		RecipientEmail: "test@example.com",
		Body:           "hello",
	}
	b, err := json.Marshal(msg)
	require.NoError(t, err)
	return b
}

// TestHandleWithRetry_MalformedJSON verifies that a message whose Value isn't valid JSON
// is skipped immediately (returns true so the offset is committed) without ever invoking
// the handler — malformed messages can't be meaningfully retried or DLQ'd.
func TestHandleWithRetry_MalformedJSON(t *testing.T) {
	var called atomic.Bool
	c := &Consumer{
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			called.Store(true)
			return nil
		},
		log:      testLogger(t),
		topic:    "test-topic",
		producer: nil,
	}

	km := kafka.Message{Value: []byte("{not valid json")}
	ok := c.handleWithRetry(context.Background(), km)

	assert.True(t, ok, "malformed message should be treated as safely skippable (commit offset)")
	assert.False(t, called.Load(), "handler must not be called for malformed JSON")
}

// TestHandleWithRetry_SucceedsAfterRetries verifies the handler is retried on failure and
// that handleWithRetry returns true (commit) as soon as the handler succeeds, having been
// invoked exactly N times for a handler that fails (N-1) times before succeeding.
func TestHandleWithRetry_SucceedsAfterRetries(t *testing.T) {
	var attempts atomic.Int32
	notificationID := uuid.New()

	c := &Consumer{
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			n := attempts.Add(1)
			assert.Equal(t, notificationID, msg.NotificationID)
			if n < 2 {
				return errors.New("transient failure")
			}
			return nil
		},
		log:      testLogger(t),
		topic:    "test-topic",
		producer: nil,
	}

	km := kafka.Message{Value: validMessageBytes(t, notificationID)}
	ok := c.handleWithRetry(context.Background(), km)

	assert.True(t, ok)
	assert.EqualValues(t, 2, attempts.Load(), "handler should be called exactly twice (1 failure + 1 success)")
}

// TestHandleWithRetry_NilProducerDropsAfterExhaustion exercises the full retry-exhaustion
// path (3 attempts, eating the real 1s+2s backoff) with c.producer == nil — the DLQ
// consumer's own configuration — and asserts the message is dropped (returns true, i.e.
// "safe to commit") rather than requeued, since there's nowhere further to send it without
// risking an infinite DLQ loop.
func TestHandleWithRetry_NilProducerDropsAfterExhaustion(t *testing.T) {
	var attempts atomic.Int32
	notificationID := uuid.New()

	c := &Consumer{
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			attempts.Add(1)
			return errors.New("permanent failure")
		},
		log:      testLogger(t),
		topic:    "test-topic",
		producer: nil,
	}

	km := kafka.Message{Value: validMessageBytes(t, notificationID)}
	ok := c.handleWithRetry(context.Background(), km)

	assert.True(t, ok, "nil-producer path must drop (commit) after exhausting retries, not loop forever")
	assert.EqualValues(t, maxRetries, attempts.Load(), "handler should be invoked exactly maxRetries times")
}

// TestHandleWithRetry_CtxCancelledMidBackoff cancels the context right after the first
// failed attempt, before the 1s backoff completes, and asserts handleWithRetry returns
// false (do not commit — shutdown mid-retry, message will be redelivered) and that the
// handler was only invoked once (the cancellation interrupts the wait, so a second attempt
// never happens).
func TestHandleWithRetry_CtxCancelledMidBackoff(t *testing.T) {
	var attempts atomic.Int32
	notificationID := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())

	c := &Consumer{
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			attempts.Add(1)
			cancel() // cancel immediately after the first attempt fails, before backoff sleep
			return errors.New("fails then ctx is cancelled")
		},
		log:      testLogger(t),
		topic:    "test-topic",
		producer: nil,
	}

	km := kafka.Message{Value: validMessageBytes(t, notificationID)}
	ok := c.handleWithRetry(ctx, km)

	assert.False(t, ok, "context cancellation mid-backoff must return false (do not commit)")
	assert.EqualValues(t, 1, attempts.Load(), "handler should only be called once before cancellation aborts the retry loop")
}

// TestHandleWithRetry_BackoffDurations is a lightweight sanity check that the package-level
// retryBackoff table has the documented 1s/2s/4s shape and length matching maxRetries, so a
// future edit to one without the other is caught by a test instead of only by behavior.
func TestHandleWithRetry_BackoffDurations(t *testing.T) {
	require.Len(t, retryBackoff, maxRetries)
	assert.Equal(t, time.Second, retryBackoff[0])
	assert.Equal(t, 2*time.Second, retryBackoff[1])
}

// TestClose verifies Close does not panic when called on a Consumer with a real
// (unconnected) kafka.Reader — exercising the simple delegation path.
func TestClose(t *testing.T) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
		GroupID: "test-group",
	})
	c := &Consumer{reader: r, log: testLogger(t), topic: "test-topic"}
	assert.NotPanics(t, func() { c.Close() })
}

// TestNew_RequiresBootstrapServers verifies the config-validation guard in New: an empty
// BootstrapServers must fail fast with a clear error instead of attempting to dial.
func TestNew_RequiresBootstrapServers(t *testing.T) {
	c, err := New(Config{}, "group", "topic", func(ctx context.Context, msg *kafkatypes.Message) error { return nil }, nil, testLogger(t))
	assert.Nil(t, c)
	assert.Error(t, err)
}
