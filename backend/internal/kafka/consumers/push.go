package consumers

import (
	"context"

	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewPushConsumer creates a consumer for the notifyx.push topic.
// Phase 6 replaces the stub handler with a real Firebase FCM call.
func NewPushConsumer(cfg Config, prod *producer.Producer, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		// TODO(phase-6): call Firebase FCM here.
		log.Infow("stub: push delivery",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"recipient_token", msg.RecipientToken,
		)
		return nil
	}

	return New(cfg, "notifyx-push", kafkatypes.TopicPush, handler, prod, log)
}
