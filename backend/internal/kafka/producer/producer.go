package producer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/tracing"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Producer wraps a kafka.Writer configured for Confluent Cloud (SASL_SSL / PLAIN).
//
// segmentio/kafka-go is used instead of the spec's confluent-kafka-go because
// confluent-kafka-go requires CGO + librdkafka headers. segmentio/kafka-go is
// pure Go, builds without system dependencies, and speaks the same Kafka wire
// protocol — fully compatible with Confluent Cloud.
// ProducerInterface is the seam used by handlers/consumers/scheduler so they can be
// unit-tested against a mock instead of a real Kafka connection.
type ProducerInterface interface {
	Publish(ctx context.Context, topic string, msg *kafkatypes.Message) error
}

type Producer struct {
	writer *kafka.Writer
	log    *logger.Logger
}

var _ ProducerInterface = (*Producer)(nil)

// New creates a producer connected to Confluent Cloud.
// bootstrapServers is the host:port from the Confluent Cloud cluster settings.
// apiKey / apiSecret are the Confluent API key and secret (used as SASL username/password).
func New(bootstrapServers, apiKey, apiSecret string, log *logger.Logger) (*Producer, error) {
	if bootstrapServers == "" {
		return nil, fmt.Errorf("KAFKA_BOOTSTRAP_SERVERS is required")
	}

	// Transport holds TLS + SASL config. Confluent Cloud requires TLS 1.2+ and
	// SASL PLAIN (API key as username, API secret as password).
	transport := &kafka.Transport{
		TLS: &tls.Config{MinVersion: tls.VersionTLS12},
		SASL: plain.Mechanism{
			Username: apiKey,
			Password: apiSecret,
		},
	}

	// Writer without a fixed topic — we set the topic per-message so one writer
	// can publish to all notifyx.* topics.
	w := &kafka.Writer{
		Addr:      kafka.TCP(bootstrapServers),
		Transport: transport,
		// Hash on notification ID, not priority — priority only has 3-4 distinct
		// values, so keying on it sends every message of a given priority to the
		// same partition regardless of partition count, which defeats the purpose
		// of having multiple partitions (no parallelism across consumers within a
		// priority class). Notification ID is high-cardinality and spreads load
		// evenly while still routing all of one notification's messages (e.g. a
		// retry republish) to the same partition for ordering.
		Balancer: &kafka.Hash{},
		// RequireAll: wait for all in-sync replicas to ack, not just the leader.
		// RequireOne risks losing a message if the leader fails right after acking
		// but before the write replicates — unacceptable for a delivery pipeline
		// that already promises retry + DLQ guarantees downstream.
		RequiredAcks: kafka.RequireAll,
		Async:        false, // synchronous — Publish blocks until broker acks
	}

	log.Infow("kafka producer created", "brokers", bootstrapServers)
	return &Producer{writer: w, log: log}, nil
}

// Publish serialises msg to JSON and writes it to topic.
// The message key is the notification ID, so all messages for one notification
// (e.g. a retry republish) land on the same partition and stay in order, while
// different notifications spread across partitions for parallelism.
func (p *Producer) Publish(ctx context.Context, topic string, msg *kafkatypes.Message) error {
	ctx, span := tracing.Tracer().Start(ctx, "kafka.publish",
		trace.WithAttributes(
			attribute.String("topic", topic),
			attribute.String("notification_id", msg.NotificationID.String()),
			attribute.String("channel", string(msg.Channel)),
		))
	defer span.End()

	payload, err := json.Marshal(msg)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal kafka message: %w", err)
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(msg.NotificationID.String()),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "notification_id", Value: []byte(msg.NotificationID.String())},
			{Key: "channel", Value: []byte(string(msg.Channel))},
		},
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("write kafka message to %s: %w", topic, err)
	}

	p.log.Infow("message published",
		"topic", topic,
		"notification_id", msg.NotificationID,
		"channel", msg.Channel,
		"priority", msg.Priority,
	)
	return nil
}

// Close flushes any buffered messages and closes the writer.
// Call defer producer.Close() in main.
func (p *Producer) Close() {
	if err := p.writer.Close(); err != nil {
		p.log.Errorw("kafka producer close error", "error", err)
	}
}
