package consumers

import (
	"context"
	"fmt"

	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/metrics"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewDLQConsumer creates a consumer for the notifyx.dlq topic.
// It persists each failed message to the dlq_messages table and marks the corresponding
// notification/delivery rows "failed" — this is the one place a terminal failure is
// recorded, since a message only reaches the DLQ after the channel consumer's retry loop
// has exhausted all attempts. Recording it here (rather than in the channel handler) means
// attempts/error_message reflect one outcome per Kafka-level delivery, not one per retry.
//
// The producer is intentionally nil — if the DLQ handler itself fails after
// all retries, the message is logged and dropped (not re-published to DLQ)
// to prevent an infinite retry loop.
func NewDLQConsumer(cfg Config, dlqRepo *postgres.DLQRepository, notifications *postgres.NotificationRepository, deliveries *postgres.NotificationDeliveryRepository, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		reason := msg.LastError
		if reason == "" {
			reason = "exhausted delivery retries (no error recorded)"
		}
		if err := dlqRepo.Create(ctx, msg, reason); err != nil {
			return fmt.Errorf("persist to dlq_messages: %w", err)
		}

		if err := deliveries.UpdateStatus(ctx, msg.DeliveryID, domain.StatusFailed, reason); err != nil {
			log.Errorw("DLQ: mark delivery failed failed", "notification_id", msg.NotificationID, "error", err)
		}
		if err := notifications.UpdateStatus(ctx, msg.NotificationID, domain.StatusFailed); err != nil {
			log.Errorw("DLQ: mark notification failed failed", "notification_id", msg.NotificationID, "error", err)
		}
		metrics.NotificationsTotal.WithLabelValues(string(msg.Channel), string(domain.StatusFailed)).Inc()

		log.Infow("DLQ message persisted",
			"notification_id", msg.NotificationID,
			"channel", msg.Channel,
		)
		return nil
	}

	return New(cfg, "notifyx-dlq", kafkatypes.TopicDLQ, handler, nil, log)
}
