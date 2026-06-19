package producer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// Producer wraps a kafka.Writer configured for Confluent Cloud (SASL_SSL / PLAIN).
//
// segmentio/kafka-go is used instead of the spec's confluent-kafka-go because
// confluent-kafka-go requires CGO + librdkafka headers. segmentio/kafka-go is
// pure Go, builds without system dependencies, and speaks the same Kafka wire
// protocol — fully compatible with Confluent Cloud.
type Producer struct {
	writer *kafka.Writer
	log    *logger.Logger
}

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
		Addr:         kafka.TCP(bootstrapServers),
		Transport:    transport,
		Balancer:     &kafka.Hash{}, // hash on message key (priority) for consistent routing
		RequiredAcks: kafka.RequireOne,
		Async:        false, // synchronous — Publish blocks until broker acks
	}

	log.Infow("kafka producer created", "brokers", bootstrapServers)
	return &Producer{writer: w, log: log}, nil
}

// Publish serialises msg to JSON and writes it to topic.
// The message key is set to the notification priority so that messages with the
// same priority always land on the same partition and are processed in order.
func (p *Producer) Publish(ctx context.Context, topic string, msg *kafkatypes.Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal kafka message: %w", err)
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		// Partition key = priority. Confluent Cloud with default partitioner
		// will hash this key, so same-priority messages land on the same partition.
		Key:   []byte(string(msg.Priority)),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "notification_id", Value: []byte(msg.NotificationID.String())},
			{Key: "channel", Value: []byte(string(msg.Channel))},
		},
	})
	if err != nil {
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
