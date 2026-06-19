package consumers

import (
	"context"

	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewEmailConsumer creates a consumer for the notifyx.email topic.
// Phase 6 replaces the stub handler with a real Resend API call.
func NewEmailConsumer(cfg Config, prod *producer.Producer, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		// TODO(phase-6): call Resend API here.
		log.Infow("stub: email delivery",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"recipient_email", msg.RecipientEmail,
			"subject", msg.Subject,
		)
		return nil
	}

	return New(cfg, "notifyx-email", kafkatypes.TopicEmail, handler, prod, log)
}
