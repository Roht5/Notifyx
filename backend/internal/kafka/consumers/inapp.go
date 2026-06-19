package consumers

import (
	"context"

	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewInAppConsumer creates a consumer for the notifyx.inapp topic.
// Phase 7 replaces the stub handler with real WebSocket delivery + presence check.
func NewInAppConsumer(cfg Config, prod *producer.Producer, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		// TODO(phase-7): check Redis presence, deliver via WebSocket or push to offline queue.
		log.Infow("stub: in-app delivery",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"recipient_id", msg.RecipientID,
		)
		return nil
	}

	return New(cfg, "notifyx-inapp", kafkatypes.TopicInApp, handler, prod, log)
}
