package consumers

import (
	"context"

	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewSMSConsumer creates a consumer for the notifyx.sms topic.
// Phase 6 replaces the stub handler with a real Fast2SMS API call.
func NewSMSConsumer(cfg Config, prod *producer.Producer, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		// TODO(phase-6): call Fast2SMS REST API here.
		log.Infow("stub: SMS delivery",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"recipient_phone", msg.RecipientPhone,
		)
		return nil
	}

	return New(cfg, "notifyx-sms", kafkatypes.TopicSMS, handler, prod, log)
}
