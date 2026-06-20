package consumers

import (
	"context"
	"fmt"
	"time"

	"github.com/rohit-bagade/notifyx/internal/channels/sms"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/metrics"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewSMSConsumer creates a consumer for the notifyx.sms topic that sends through Fast2SMS.
// See NewEmailConsumer for why terminal "failed" status is recorded in the DLQ consumer instead
// of here.
func NewSMSConsumer(cfg Config, prod producer.ProducerInterface, client sms.Sender, deliveries postgres.NotificationDeliveryRepositoryInterface, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		if msg.RecipientPhone == "" {
			return fmt.Errorf("sms handler: missing recipient_phone")
		}

		start := time.Now()
		requestID, err := client.Send(ctx, msg.RecipientPhone, msg.Body)
		metrics.ChannelSendDuration.WithLabelValues(string(domain.ChannelSMS)).Observe(time.Since(start).Seconds())
		if err != nil {
			return fmt.Errorf("fast2sms send failed: %w", err)
		}

		if err := deliveries.UpdateStatus(ctx, msg.DeliveryID, domain.StatusDelivered, ""); err != nil {
			log.Errorw("sms: mark delivered failed", "notification_id", msg.NotificationID, "error", err)
		}
		metrics.NotificationsTotal.WithLabelValues(string(domain.ChannelSMS), string(domain.StatusDelivered)).Inc()

		log.Infow("sms delivered",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"provider_request_id", requestID,
		)
		return nil
	}

	return New(cfg, "notifyx-sms", kafkatypes.TopicSMS, handler, prod, log)
}
