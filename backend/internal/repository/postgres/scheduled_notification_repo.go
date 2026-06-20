package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// ScheduledNotificationRepository handles all DB operations for the scheduled_notifications
// table — the cron-poll side of Phase 9. The notification itself (with status=pending and
// scheduled_at set) is persisted separately by NotificationRepository.Create; this table just
// tracks which notification IDs are due and whether they've already been fired.
// ScheduledNotificationRepositoryInterface is the seam used by the scheduler so it can
// be unit-tested against a mock instead of a real Postgres connection.
type ScheduledNotificationRepositoryInterface interface {
	Create(ctx context.Context, notificationID uuid.UUID, scheduledAt time.Time) (*domain.ScheduledNotification, error)
	GetDueUnfired(ctx context.Context, now time.Time, limit int) ([]*domain.ScheduledNotification, error)
	MarkFired(ctx context.Context, id uuid.UUID) error
}

type ScheduledNotificationRepository struct {
	pool Executor
}

var _ ScheduledNotificationRepositoryInterface = (*ScheduledNotificationRepository)(nil)

func NewScheduledNotificationRepository(pool Executor) *ScheduledNotificationRepository {
	return &ScheduledNotificationRepository{pool: pool}
}

func (r *ScheduledNotificationRepository) Create(ctx context.Context, notificationID uuid.UUID, scheduledAt time.Time) (*domain.ScheduledNotification, error) {
	var s domain.ScheduledNotification
	err := r.pool.QueryRow(ctx,
		`INSERT INTO scheduled_notifications (notification_id, scheduled_at)
		 VALUES ($1, $2)
		 RETURNING id, notification_id, scheduled_at, fired, created_at`,
		notificationID, scheduledAt,
	).Scan(&s.ID, &s.NotificationID, &s.ScheduledAt, &s.Fired, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create scheduled notification: %w", err)
	}
	return &s, nil
}

// GetDueUnfired returns up to limit unfired scheduled_notifications rows whose scheduled_at
// has passed, oldest-due first. Backed by the partial index on (scheduled_at) WHERE fired = FALSE.
func (r *ScheduledNotificationRepository) GetDueUnfired(ctx context.Context, now time.Time, limit int) ([]*domain.ScheduledNotification, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, notification_id, scheduled_at, fired, created_at
		 FROM scheduled_notifications
		 WHERE fired = FALSE AND scheduled_at <= $1
		 ORDER BY scheduled_at ASC
		 LIMIT $2`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get due scheduled notifications: %w", err)
	}
	defer rows.Close()

	var due []*domain.ScheduledNotification
	for rows.Next() {
		var s domain.ScheduledNotification
		if err := rows.Scan(&s.ID, &s.NotificationID, &s.ScheduledAt, &s.Fired, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan scheduled notification: %w", err)
		}
		due = append(due, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled notifications: %w", err)
	}
	return due, nil
}

// MarkFired flips fired = true so a later poll never re-publishes this row.
func (r *ScheduledNotificationRepository) MarkFired(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE scheduled_notifications SET fired = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark scheduled notification fired: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
