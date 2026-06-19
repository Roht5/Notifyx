package consumers

import (
	"context"
	"fmt"
	"time"

	"github.com/rohit-bagade/notifyx/internal/channels/push"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/metrics"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewPushConsumer creates a consumer for the notifyx.push topic that sends through Firebase FCM.
// See NewEmailConsumer for why terminal "failed" status is recorded in the DLQ consumer instead
// of here.
func NewPushConsumer(cfg Config, prod *producer.Producer, client *push.Client, deliveries *postgres.NotificationDeliveryRepository, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		if msg.RecipientToken == "" {
			return fmt.Errorf("push handler: missing recipient_token")
		}

		start := time.Now()
		name, err := client.Send(ctx, msg.RecipientToken, msg.Subject, msg.Body)
		metrics.ChannelSendDuration.WithLabelValues(string(domain.ChannelPush)).Observe(time.Since(start).Seconds())
		if err != nil {
			return fmt.Errorf("fcm send failed: %w", err)
		}

		if err := deliveries.UpdateStatus(ctx, msg.DeliveryID, domain.StatusDelivered, ""); err != nil {
			log.Errorw("push: mark delivered failed", "notification_id", msg.NotificationID, "error", err)
		}
		metrics.NotificationsTotal.WithLabelValues(string(domain.ChannelPush), string(domain.StatusDelivered)).Inc()

		log.Infow("push delivered",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"provider_message_name", name,
		)
		return nil
	}

	return New(cfg, "notifyx-push", kafkatypes.TopicPush, handler, prod, log)
}
