// Package scheduler polls scheduled_notifications for due-but-unfired rows and publishes
// them to Kafka, mirroring the ratelimit.Replayer background-worker pattern.
package scheduler

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

// batchSize caps how many due rows are polled per tick.
const batchSize = 100

// maxConcurrentFires bounds how many scheduled notifications Scheduler publishes at once
// per tick.
const maxConcurrentFires = 16

// Scheduler periodically publishes notifications whose scheduled_at has passed. Producer
// may be nil (Kafka not configured) — in that case the row is deliberately left unfired so
// a retry once Kafka is wired up will still pick it up, instead of silently losing it.
type Scheduler struct {
	scheduled     postgres.ScheduledNotificationRepositoryInterface
	notifications postgres.NotificationRepositoryInterface
	deliveries    postgres.NotificationDeliveryRepositoryInterface
	producer      producer.ProducerInterface
	log           *logger.Logger
}

func New(
	scheduled postgres.ScheduledNotificationRepositoryInterface,
	notifications postgres.NotificationRepositoryInterface,
	deliveries postgres.NotificationDeliveryRepositoryInterface,
	prod producer.ProducerInterface,
	log *logger.Logger,
) *Scheduler {
	return &Scheduler{scheduled: scheduled, notifications: notifications, deliveries: deliveries, producer: prod, log: log}
}

// Run ticks every interval until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tickOnce(ctx)
		}
	}
}

func (s *Scheduler) tickOnce(ctx context.Context) {
	due, err := s.scheduled.GetDueUnfired(ctx, time.Now(), batchSize)
	if err != nil {
		s.log.Errorw("scheduler: list due notifications failed", "error", err)
		return
	}

	sem := make(chan struct{}, maxConcurrentFires)
	var wg sync.WaitGroup
	for _, sn := range due {
		wg.Add(1)
		sem <- struct{}{}
		go func(sn *domain.ScheduledNotification) {
			defer wg.Done()
			defer func() { <-sem }()
			s.fireOne(ctx, sn)
		}(sn)
	}
	wg.Wait()
}

// fireOne loads the underlying notification, publishes it (if Kafka is configured), and
// marks the row fired only once publish succeeds — if Kafka isn't configured the row is
// left unfired so a future tick (once KAFKA_BOOTSTRAP_SERVERS is set) still publishes it,
// rather than silently dropping it the way an immediate send's createAndQueue does.
func (s *Scheduler) fireOne(ctx context.Context, sn *domain.ScheduledNotification) {
	n, err := s.notifications.GetByID(ctx, sn.NotificationID)
	if err != nil {
		s.log.Errorw("scheduler: load notification failed", "scheduled_id", sn.ID, "notification_id", sn.NotificationID, "error", err)
		return
	}

	deliveries, err := s.deliveries.GetByNotificationID(ctx, n.ID)
	if err != nil || len(deliveries) == 0 {
		s.log.Errorw("scheduler: missing delivery row", "notification_id", n.ID, "error", err)
		return
	}
	delivery := deliveries[0]

	if s.producer == nil {
		s.log.Warnw("scheduler: kafka not configured — scheduled notification due but not published", "notification_id", n.ID)
		return
	}

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
	if err := s.producer.Publish(ctx, kafkatypes.TopicForChannel(n.Channel), msg); err != nil {
		s.log.Errorw("scheduler: publish failed", "notification_id", n.ID, "error", err)
		return
	}

	if err := s.notifications.UpdateStatus(ctx, n.ID, domain.StatusQueued); err != nil {
		s.log.Errorw("scheduler: update notification status failed", "notification_id", n.ID, "error", err)
	}
	if err := s.deliveries.SetStatus(ctx, delivery.ID, domain.StatusQueued); err != nil {
		s.log.Errorw("scheduler: update delivery status failed", "delivery_id", delivery.ID, "error", err)
	}
	if err := s.scheduled.MarkFired(ctx, sn.ID); err != nil {
		s.log.Errorw("scheduler: mark fired failed", "scheduled_id", sn.ID, "error", err)
	}
	s.log.Infow("scheduler: scheduled notification published", "notification_id", n.ID)
}
