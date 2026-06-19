package consumers

import (
	"context"
	"fmt"

	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NewDLQConsumer creates a consumer for the notifyx.dlq topic.
// It persists each failed message to the dlq_messages table.
//
// The producer is intentionally nil — if the DLQ handler itself fails after
// all retries, the message is logged and dropped (not re-published to DLQ)
// to prevent an infinite retry loop.
func NewDLQConsumer(cfg Config, dlqRepo *postgres.DLQRepository, log *logger.Logger) (*Consumer, error) {
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		reason := msg.LastError
		if reason == "" {
			reason = "exhausted delivery retries (no error recorded)"
		}
		if err := dlqRepo.Create(ctx, msg, reason); err != nil {
			return fmt.Errorf("persist to dlq_messages: %w", err)
		}
		log.Infow("DLQ message persisted",
			"notification_id", msg.NotificationID,
			"channel", msg.Channel,
		)
		return nil
	}

	return New(cfg, "notifyx-dlq", kafkatypes.TopicDLQ, handler, nil, log)
}
