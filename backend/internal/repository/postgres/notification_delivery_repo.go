package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// NotificationDeliveryRepository handles all DB operations for the notification_deliveries table.
// pool is an Executor (not *pgxpool.Pool directly) so the same repository type
// can also be constructed against a transaction — see WithTx in db.go.
type NotificationDeliveryRepository struct {
	pool Executor
}

func NewNotificationDeliveryRepository(pool Executor) *NotificationDeliveryRepository {
	return &NotificationDeliveryRepository{pool: pool}
}

// Create inserts a delivery row. An empty status defaults to 'queued' (the column default);
// the rate-limited send path passes domain.StatusQueuedRateLimited instead.
func (r *NotificationDeliveryRepository) Create(ctx context.Context, notificationID uuid.UUID, channel domain.Channel, status domain.Status) (*domain.NotificationDelivery, error) {
	if status == "" {
		status = domain.StatusQueued
	}
	var d domain.NotificationDelivery
	var ch, st string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO notification_deliveries (notification_id, channel, status)
		 VALUES ($1, $2, $3)
		 RETURNING id, notification_id, channel, status, attempts, created_at`,
		notificationID, string(channel), string(status),
	).Scan(&d.ID, &d.NotificationID, &ch, &st, &d.Attempts, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create notification delivery: %w", err)
	}
	d.Channel = domain.Channel(ch)
	d.Status = domain.Status(st)
	return &d, nil
}

// SetStatus is a plain status transition with no side effects — unlike UpdateStatus, it
// doesn't bump attempts or touch delivered_at. Used by the rate-limit replayer to move a
// delivery from queued_rate_limited to queued once its tenant's window clears; that's a
// requeue, not a delivery attempt.
func (r *NotificationDeliveryRepository) SetStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	tag, err := r.pool.Exec(ctx, `UPDATE notification_deliveries SET status = $1 WHERE id = $2`, string(status), id)
	if err != nil {
		return fmt.Errorf("set delivery status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus records the outcome of one delivery attempt: increments the attempt
// counter, sets the terminal status and error message, and stamps delivered_at on success.
func (r *NotificationDeliveryRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, errorMessage string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notification_deliveries
		 SET status = $1,
		     error_message = $2,
		     attempts = attempts + 1,
		     delivered_at = CASE WHEN $1 = 'delivered' THEN NOW() ELSE delivered_at END
		 WHERE id = $3`,
		string(status), errorMessage, id,
	)
	if err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByNotificationID returns every delivery attempt recorded for a notification, oldest first.
func (r *NotificationDeliveryRepository) GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*domain.NotificationDelivery, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, notification_id, channel, status, attempts, error_message, delivered_at, created_at
		 FROM notification_deliveries WHERE notification_id = $1 ORDER BY created_at ASC`,
		notificationID,
	)
	if err != nil {
		return nil, fmt.Errorf("get deliveries by notification id: %w", err)
	}
	defer rows.Close()

	var deliveries []*domain.NotificationDelivery
	for rows.Next() {
		var d domain.NotificationDelivery
		var ch, status string
		var errorMessage *string
		if err := rows.Scan(&d.ID, &d.NotificationID, &ch, &status, &d.Attempts, &errorMessage, &d.DeliveredAt, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification delivery: %w", err)
		}
		d.Channel = domain.Channel(ch)
		d.Status = domain.Status(status)
		if errorMessage != nil {
			d.ErrorMessage = *errorMessage
		}
		deliveries = append(deliveries, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification deliveries: %w", err)
	}
	return deliveries, nil
}
