package consumers

import (
	"context"
	"fmt"
	"time"

	"github.com/rohit-bagade/notifyx/internal/channels/email"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/metrics"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewEmailConsumer creates a consumer for the notifyx.email topic that sends through Resend.
// On success it marks the delivery row "delivered". On failure it returns the error and lets
// the shared retry/DLQ machinery in Consumer handle retries — the DLQ consumer is the one
// that records the terminal "failed" status, so attempts/error_message reflect one row per
// Kafka-level delivery outcome rather than incrementing on every retry.
func NewEmailConsumer(cfg Config, prod producer.ProducerInterface, client email.Sender, deliveries postgres.NotificationDeliveryRepositoryInterface, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		if msg.RecipientEmail == "" {
			return fmt.Errorf("email handler: missing recipient_email")
		}

		start := time.Now()
		id, err := client.Send(ctx, msg.RecipientEmail, msg.Subject, msg.Body)
		metrics.ChannelSendDuration.WithLabelValues(string(domain.ChannelEmail)).Observe(time.Since(start).Seconds())
		if err != nil {
			return fmt.Errorf("resend send failed: %w", err)
		}

		if err := deliveries.UpdateStatus(ctx, msg.DeliveryID, domain.StatusDelivered, ""); err != nil {
			log.Errorw("email: mark delivered failed", "notification_id", msg.NotificationID, "error", err)
		}
		metrics.NotificationsTotal.WithLabelValues(string(domain.ChannelEmail), string(domain.StatusDelivered)).Inc()

		log.Infow("email delivered",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"recipient_email", msg.RecipientEmail,
			"provider_message_id", id,
		)
		return nil
	}

	return New(cfg, "notifyx-email", kafkatypes.TopicEmail, handler, prod, log)
}
