package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/offlinequeue"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/internal/ws"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// inAppPayload is what gets pushed over the WebSocket / stored in the offline queue —
// a small client-facing subset of kafkatypes.Message, not the whole envelope.
type inAppPayload struct {
	NotificationID string         `json:"notification_id"`
	Subject        string         `json:"subject,omitempty"`
	Body           string         `json:"body"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	SentAt         time.Time      `json:"sent_at"`
}

// NewInAppConsumer creates a consumer for the notifyx.inapp topic. If the recipient has a
// live WebSocket connection it delivers immediately; otherwise, when queue is configured
// (REDIS_URL set), it persists the message for delivery on next reconnect — that's recorded
// as StatusQueued via SetStatus (not UpdateStatus) since it's not itself a delivery attempt,
// mirroring how the rate-limit replayer marks queued_rate_limited rows. With no queue
// configured, it returns an error so the existing retry/DLQ machinery records the terminal
// failure, the same as any other channel's permanent delivery failure.
// NewInAppConsumer takes hub/queue as interfaces so it can be unit-tested against mocks.
// Callers passing a concrete *offlinequeue.Queue must pass a nil *offlinequeue.Queue (not a
// pre-converted nil interface) when Redis isn't configured — Go's nil-interface semantics
// mean a nil *offlinequeue.Queue boxed into the Pusher interface is itself non-nil, so we
// detect that specific case below to preserve the original "no queue configured" behavior.
func NewInAppConsumer(cfg Config, prod producer.ProducerInterface, hub ws.Notifier, queue offlinequeue.Pusher, deliveries postgres.NotificationDeliveryRepositoryInterface, log *logger.Logger) (*Consumer, error) {
	if q, ok := queue.(*offlinequeue.Queue); ok && q == nil {
		queue = nil
	}
	handler := func(ctx context.Context, msg *kafkatypes.Message) error {
		if msg.RecipientID == "" {
			return fmt.Errorf("inapp handler: missing recipient_id")
		}

		payload, err := json.Marshal(inAppPayload{
			NotificationID: msg.NotificationID.String(),
			Subject:        msg.Subject,
			Body:           msg.Body,
			Metadata:       msg.Metadata,
			SentAt:         time.Now().UTC(),
		})
		if err != nil {
			return fmt.Errorf("inapp handler: marshal payload failed: %w", err)
		}

		if hub.SendToUser(msg.TenantID.String(), msg.RecipientID, payload) {
			if err := deliveries.UpdateStatus(ctx, msg.DeliveryID, domain.StatusDelivered, ""); err != nil {
				log.Errorw("inapp: mark delivered failed", "notification_id", msg.NotificationID, "error", err)
			}
			log.Infow("inapp delivered over websocket",
				"notification_id", msg.NotificationID,
				"tenant_id", msg.TenantID,
				"recipient_id", msg.RecipientID,
			)
			return nil
		}

		if queue == nil {
			return fmt.Errorf("inapp handler: recipient offline and no offline queue configured")
		}

		if err := queue.Push(ctx, msg.TenantID.String(), msg.RecipientID, payload); err != nil {
			return fmt.Errorf("inapp handler: offline queue push failed: %w", err)
		}

		if err := deliveries.SetStatus(ctx, msg.DeliveryID, domain.StatusQueued); err != nil {
			log.Errorw("inapp: mark queued failed", "notification_id", msg.NotificationID, "error", err)
		}
		log.Infow("inapp queued for offline delivery",
			"notification_id", msg.NotificationID,
			"tenant_id", msg.TenantID,
			"recipient_id", msg.RecipientID,
		)
		return nil
	}

	return New(cfg, "notifyx-inapp", kafkatypes.TopicInApp, handler, prod, log)
}
