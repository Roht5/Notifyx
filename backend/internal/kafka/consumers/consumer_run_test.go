package consumers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"

	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
)

// mockReader mocks the Reader seam used by Consumer.Run.
type mockReader struct {
	mock.Mock
}

func (m *mockReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	args := m.Called(ctx)
	km, _ := args.Get(0).(kafka.Message)
	return km, args.Error(1)
}

func (m *mockReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	args := m.Called(ctx, msgs)
	return args.Error(0)
}

func (m *mockReader) Close() error {
	args := m.Called()
	return args.Error(0)
}

var _ Reader = (*mockReader)(nil)

// TestRun_HandlesMessageAndCommits verifies the success path: FetchMessage returns a
// valid message, the handler succeeds, and Run commits the offset. The second
// FetchMessage call returns context.Canceled (simulating shutdown after ctx is
// cancelled) so Run exits cleanly without an infinite loop.
func TestRun_HandlesMessageAndCommits(t *testing.T) {
	notificationID := uuid.New()
	km := kafka.Message{Value: validMessageBytes(t, notificationID), Offset: 1}

	reader := &mockReader{}
	ctx, cancel := context.WithCancel(context.Background())

	reader.On("FetchMessage", mock.Anything).Return(km, nil).Once()
	reader.On("FetchMessage", mock.Anything).Return(kafka.Message{}, context.Canceled).Run(func(args mock.Arguments) {
		cancel()
	})
	reader.On("CommitMessages", mock.Anything, mock.Anything).Return(nil)
	reader.On("Close").Return(nil)

	c := &Consumer{
		reader: reader,
		topic:  "test-topic",
		log:    testLogger(t),
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			return nil
		},
	}

	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	reader.AssertCalled(t, "CommitMessages", mock.Anything, mock.Anything)
}

// TestRun_FetchErrorBacksOffThenShutsDown verifies that a non-context-cancellation
// FetchMessage error triggers the fetchErrorBackoff wait rather than a tight loop, and
// that Run exits cleanly once ctx is cancelled during that wait.
func TestRun_FetchErrorBacksOffThenShutsDown(t *testing.T) {
	reader := &mockReader{}
	ctx, cancel := context.WithCancel(context.Background())

	reader.On("FetchMessage", mock.Anything).Return(kafka.Message{}, errors.New("broker unreachable"))
	reader.On("Close").Return(nil)

	c := &Consumer{
		reader:  reader,
		topic:   "test-topic",
		log:     testLogger(t),
		handler: func(ctx context.Context, msg *kafkatypes.Message) error { return nil },
	}

	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	// Cancel almost immediately; Run should be sitting in the fetchErrorBackoff select
	// (real 2s constant) and the ctx.Done() case must win without waiting out the backoff.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancellation during fetch-error backoff")
	}
}

// TestRun_CommitFailureLogsAndContinues verifies a commit failure (post-success-handle)
// is logged but does not stop the loop — the next FetchMessage call observes ctx
// cancellation and Run exits cleanly.
func TestRun_CommitFailureLogsAndContinues(t *testing.T) {
	notificationID := uuid.New()
	km := kafka.Message{Value: validMessageBytes(t, notificationID), Offset: 1}

	reader := &mockReader{}
	ctx, cancel := context.WithCancel(context.Background())

	reader.On("FetchMessage", mock.Anything).Return(km, nil).Once()
	reader.On("FetchMessage", mock.Anything).Return(kafka.Message{}, context.Canceled).Run(func(args mock.Arguments) {
		cancel()
	})
	reader.On("CommitMessages", mock.Anything, mock.Anything).Return(errors.New("commit failed"))
	reader.On("Close").Return(nil)

	c := &Consumer{
		reader:  reader,
		topic:   "test-topic",
		log:     testLogger(t),
		handler: func(ctx context.Context, msg *kafkatypes.Message) error { return nil },
	}

	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

// TestRun_HandlerFailureExhaustionLeavesOffsetUncommitted verifies that when
// handleWithRetry returns false (here forced via a producer that fails DLQ publish), Run
// does not call CommitMessages for that message and continues to the next fetch.
func TestRun_HandlerFailureExhaustionLeavesOffsetUncommitted(t *testing.T) {
	withShrunkBackoff(t)
	notificationID := uuid.New()
	km := kafka.Message{Value: validMessageBytes(t, notificationID), Offset: 1}

	reader := &mockReader{}
	ctx, cancel := context.WithCancel(context.Background())

	prod := &mockProducerDLQ{}
	prod.On("Publish", mock.Anything, kafkatypes.TopicDLQ, mock.Anything).Return(assertErr)

	reader.On("FetchMessage", mock.Anything).Return(km, nil).Once()
	reader.On("FetchMessage", mock.Anything).Return(kafka.Message{}, context.Canceled).Run(func(args mock.Arguments) {
		cancel()
	})
	reader.On("Close").Return(nil)

	c := &Consumer{
		reader:   reader,
		topic:    "test-topic",
		log:      testLogger(t),
		producer: prod,
		handler: func(ctx context.Context, msg *kafkatypes.Message) error {
			return assertErr
		},
	}

	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	reader.AssertNotCalled(t, "CommitMessages", mock.Anything, mock.Anything)
}
