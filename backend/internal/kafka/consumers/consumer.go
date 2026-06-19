package consumers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

const maxRetries = 3

// fetchErrorBackoff is how long Run waits after a non-shutdown FetchMessage error
// before retrying — without it, a persistent broker/network problem turns into a
// tight CPU- and log-flooding loop.
const fetchErrorBackoff = 2 * time.Second

// backoff durations between successive retry attempts.
// Attempt 1 fails → wait 1s → attempt 2 fails → wait 2s → attempt 3 fails → DLQ.
var retryBackoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// HandlerFunc processes one decoded message. Return a non-nil error to trigger
// the retry loop; return nil to mark the message as successfully processed.
type HandlerFunc func(ctx context.Context, msg *kafkatypes.Message) error

// Consumer is a reusable Kafka consumer with built-in retry and DLQ fallback.
// Every channel consumer (email, push, sms, inapp) is an instance of this type
// with a different topic, group ID, and HandlerFunc.
type Consumer struct {
	reader   *kafka.Reader
	producer *producer.Producer // nil for the DLQ consumer itself (prevents infinite loops)
	topic    string
	handler  HandlerFunc
	log      *logger.Logger
}

// Config holds the connection details shared by all consumers.
type Config struct {
	BootstrapServers string
	APIKey           string
	APISecret        string
}

// New creates a Consumer subscribed to topic with the given consumer group ID.
// prod may be nil — when nil, messages that exhaust all retries are logged and
// dropped rather than re-published to the DLQ (used by the DLQ consumer itself).
func New(cfg Config, groupID, topic string, handler HandlerFunc, prod *producer.Producer, log *logger.Logger) (*Consumer, error) {
	if cfg.BootstrapServers == "" {
		return nil, fmt.Errorf("KAFKA_BOOTSTRAP_SERVERS is required")
	}

	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS:       &tls.Config{MinVersion: tls.VersionTLS12},
		SASLMechanism: plain.Mechanism{
			Username: cfg.APIKey,
			Password: cfg.APISecret,
		},
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{cfg.BootstrapServers},
		Topic:   topic,
		GroupID: groupID,
		Dialer:  dialer,
		// Commit offsets manually — we only advance past a message after we have
		// either processed it successfully or sent it to the DLQ.
		CommitInterval: 0,
		MinBytes:       1,
		MaxBytes:       10e6, // 10 MB
		StartOffset:    kafka.FirstOffset,
		// Log Kafka internals at debug level only — avoids noise in production.
		Logger:      kafka.LoggerFunc(func(msg string, args ...interface{}) {}),
		ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {}),
	})

	return &Consumer{
		reader:   r,
		producer: prod,
		topic:    topic,
		handler:  handler,
		log:      log,
	}, nil
}

// Run starts the consumer loop and blocks until ctx is cancelled.
// Call this in a goroutine: go consumer.Run(ctx)
func (c *Consumer) Run(ctx context.Context) {
	c.log.Infow("consumer started", "topic", c.topic)
	defer func() {
		c.reader.Close()
		c.log.Infow("consumer stopped", "topic", c.topic)
	}()

	for {
		// FetchMessage blocks until a message arrives or ctx is cancelled.
		km, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // clean shutdown
			}
			c.log.Errorw("fetch message failed", "topic", c.topic, "error", err)
			select {
			case <-time.After(fetchErrorBackoff):
			case <-ctx.Done():
				return
			}
			continue
		}

		// Only commit when the message was either handled successfully or safely
		// handed off to the DLQ. Committing unconditionally (the old behavior) lost
		// messages whenever a shutdown interrupted retries or the DLQ publish itself
		// failed — the offset advanced past a message nobody ever durably recorded.
		if !c.handleWithRetry(ctx, km) {
			if ctx.Err() != nil {
				return // shutdown mid-retry — leave the offset uncommitted, redeliver on restart
			}
			c.log.Errorw("message not safely handled — leaving offset uncommitted",
				"topic", c.topic, "offset", km.Offset)
			continue
		}

		if err := c.reader.CommitMessages(ctx, km); err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Errorw("commit offset failed", "topic", c.topic, "error", err)
		}
	}
}

// handleWithRetry processes km, retrying on failure, and reports whether it's safe
// to commit the offset: true on success, on a malformed/unprocessable message, or
// once the message has been durably published to the DLQ; false if retries were
// exhausted and the DLQ publish itself failed — committing in that case would lose
// the message permanently with no record anywhere.
func (c *Consumer) handleWithRetry(ctx context.Context, km kafka.Message) bool {
	var msg kafkatypes.Message
	if err := json.Unmarshal(km.Value, &msg); err != nil {
		// Malformed message — cannot retry or DLQ meaningfully, skip it.
		c.log.Errorw("unmarshal failed — skipping message",
			"topic", c.topic,
			"offset", km.Offset,
			"error", err,
		)
		return true
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = c.handler(ctx, &msg)
		if lastErr == nil {
			return true // success
		}

		c.log.Warnw("handler failed",
			"topic", c.topic,
			"notification_id", msg.NotificationID,
			"attempt", attempt,
			"max_retries", maxRetries,
			"error", lastErr,
		)

		if attempt < maxRetries {
			select {
			case <-time.After(retryBackoff[attempt-1]):
			case <-ctx.Done():
				return false // shutdown mid-retry, do not commit
			}
		}
	}

	// All retries exhausted.
	c.log.Errorw("all retries exhausted",
		"topic", c.topic,
		"notification_id", msg.NotificationID,
		"error", lastErr,
	)

	if c.producer == nil {
		// DLQ consumer itself — drop to avoid an infinite DLQ loop. This is a
		// terminal drop, not data loss we can recover from by holding the offset.
		c.log.Errorw("DLQ handler failed after retries — dropping message to prevent loop",
			"notification_id", msg.NotificationID,
		)
		return true
	}

	msg.LastError = lastErr.Error()
	if err := c.producer.Publish(ctx, kafkatypes.TopicDLQ, &msg); err != nil {
		c.log.Errorw("DLQ publish failed — message will be redelivered",
			"notification_id", msg.NotificationID,
			"error", err,
		)
		return false
	}
	return true
}

// Close stops the underlying reader. Prefer cancelling the context passed to Run;
// this is a safety net for callers that don't use context cancellation.
func (c *Consumer) Close() {
	c.reader.Close()
}
