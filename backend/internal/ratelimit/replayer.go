package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// batchSize caps how many queued_rate_limited notifications are re-checked per tick,
// so one slow tick can't grow unbounded as the queue backs up.
const batchSize = 100

// maxConcurrentReplays bounds how many notifications replayOne processes at once — each
// one is an independent rate-check/publish/status-update pipeline, so running them
// concurrently (rather than one at a time) shrinks how long one tick blocks the
// replayer's background goroutine.
const maxConcurrentReplays = 16

// Replayer periodically re-checks notifications that were persisted as
// queued_rate_limited, publishing them to Kafka once their tenant's rate limit window
// has room again. Producer may be nil (Kafka not configured) — in that case the status
// transition still happens but publishing is skipped, the same convention used for a
// fresh send in the handler.
type Replayer struct {
	notifications *postgres.NotificationRepository
	deliveries    *postgres.NotificationDeliveryRepository
	limiter       *Limiter
	producer      *producer.Producer
	log           *logger.Logger
}

func NewReplayer(
	notifications *postgres.NotificationRepository,
	deliveries *postgres.NotificationDeliveryRepository,
	limiter *Limiter,
	prod *producer.Producer,
	log *logger.Logger,
) *Replayer {
	return &Replayer{notifications: notifications, deliveries: deliveries, limiter: limiter, producer: prod, log: log}
}

// Run ticks every interval until ctx is cancelled.
func (r *Replayer) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.replayOnce(ctx)
		}
	}
}

func (r *Replayer) replayOnce(ctx context.Context) {
	candidates, err := r.notifications.GetByStatus(ctx, domain.StatusQueuedRateLimited, batchSize)
	if err != nil {
		r.log.Errorw("replay: list rate-limited notifications failed", "error", err)
		return
	}

	sem := make(chan struct{}, maxConcurrentReplays)
	var wg sync.WaitGroup
	for _, n := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(n *domain.Notification) {
			defer wg.Done()
			defer func() { <-sem }()
			r.replayOne(ctx, n)
		}(n)
	}
	wg.Wait()
}

func (r *Replayer) replayOne(ctx context.Context, n *domain.Notification) {
	allowed, err := r.limiter.Allow(ctx, n.TenantID, n.Channel)
	if err != nil {
		r.log.Errorw("replay: rate limit check failed", "notification_id", n.ID, "error", err)
		return
	}
	if !allowed {
		return // still rate limited — try again next tick
	}

	deliveries, err := r.deliveries.GetByNotificationID(ctx, n.ID)
	if err != nil || len(deliveries) == 0 {
		r.log.Errorw("replay: missing delivery row", "notification_id", n.ID, "error", err)
		return
	}
	delivery := deliveries[0]

	if r.producer != nil {
		msg := &kafkatypes.Message{
			NotificationID: n.ID,
			DeliveryID:     delivery.ID,
			TenantID:       n.TenantID,
			Channel:        n.Channel,
			Priority:       n.Priority,
			RecipientID:    n.RecipientID,
			RecipientEmail: n.RecipientEmail,
			RecipientPhone: n.RecipientPhone,
			RecipientToken: n.RecipientToken,
			Subject:        n.Subject,
			Body:           n.Body,
			Metadata:       n.Metadata,
		}
		if err := r.producer.Publish(ctx, kafkatypes.TopicForChannel(n.Channel), msg); err != nil {
			r.log.Errorw("replay: publish failed", "notification_id", n.ID, "error", err)
			return
		}
	} else {
		r.log.Warnw("replay: kafka not configured — notification requeued but not published", "notification_id", n.ID)
	}

	if err := r.notifications.UpdateStatus(ctx, n.ID, domain.StatusQueued); err != nil {
		r.log.Errorw("replay: update notification status failed", "notification_id", n.ID, "error", err)
	}
	if err := r.deliveries.SetStatus(ctx, delivery.ID, domain.StatusQueued); err != nil {
		r.log.Errorw("replay: update delivery status failed", "delivery_id", delivery.ID, "error", err)
	}
	r.log.Infow("replay: notification requeued after rate limit cleared", "notification_id", n.ID)
}
