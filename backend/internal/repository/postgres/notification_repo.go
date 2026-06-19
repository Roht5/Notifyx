package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// NotificationRepository handles all DB operations for the notifications table.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

// CreateNotificationParams holds the fields needed to persist a new notification.
// template_id, idempotency_key, and scheduled_at are deliberately absent — those are
// populated by the Phase 8 (templates) and Phase 9 (scheduling) features.
type CreateNotificationParams struct {
	TenantID       uuid.UUID
	Channel        domain.Channel
	Priority       domain.Priority
	RecipientID    string
	RecipientEmail string
	RecipientPhone string
	RecipientToken string
	Subject        string
	Body           string
	Metadata       map[string]any
}

func (r *NotificationRepository) Create(ctx context.Context, p CreateNotificationParams) (*domain.Notification, error) {
	if p.Metadata == nil {
		p.Metadata = map[string]any{}
	}

	var n domain.Notification
	var channel, priority, status string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO notifications
			(tenant_id, channel, priority, recipient_id, recipient_email, recipient_phone, recipient_token, subject, body, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, tenant_id, channel, priority, status, recipient_id, recipient_email, recipient_phone,
		           recipient_token, subject, body, metadata, created_at, expires_at`,
		p.TenantID, string(p.Channel), string(p.Priority), p.RecipientID, p.RecipientEmail, p.RecipientPhone,
		p.RecipientToken, p.Subject, p.Body, p.Metadata,
	).Scan(
		&n.ID, &n.TenantID, &channel, &priority, &status, &n.RecipientID, &n.RecipientEmail, &n.RecipientPhone,
		&n.RecipientToken, &n.Subject, &n.Body, &n.Metadata, &n.CreatedAt, &n.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}
	n.Channel = domain.Channel(channel)
	n.Priority = domain.Priority(priority)
	n.Status = domain.Status(status)
	return &n, nil
}

func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	var n domain.Notification
	var channel, priority, status string
	var idempotencyKey *string
	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, channel, priority, status, recipient_id, recipient_email, recipient_phone,
		        recipient_token, template_id, subject, body, metadata, idempotency_key, scheduled_at, created_at, expires_at
		 FROM notifications WHERE id = $1`,
		id,
	).Scan(
		&n.ID, &n.TenantID, &channel, &priority, &status, &n.RecipientID, &n.RecipientEmail, &n.RecipientPhone,
		&n.RecipientToken, &n.TemplateID, &n.Subject, &n.Body, &n.Metadata, &idempotencyKey, &n.ScheduledAt,
		&n.CreatedAt, &n.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get notification by id: %w", err)
	}
	n.Channel = domain.Channel(channel)
	n.Priority = domain.Priority(priority)
	n.Status = domain.Status(status)
	if idempotencyKey != nil {
		n.IdempotencyKey = *idempotencyKey
	}
	return &n, nil
}

// NotificationFilter narrows GetByTenantID — zero values mean "no filter" on that field.
type NotificationFilter struct {
	Channel domain.Channel
	Status  domain.Status
	From    *time.Time
	To      *time.Time
	Limit   int
	Offset  int
}

// GetByTenantID returns a page of notifications for tenantID along with the total
// matching row count (for pagination), newest first.
func (r *NotificationRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID, f NotificationFilter) ([]*domain.Notification, int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, channel, priority, status, recipient_id, recipient_email, recipient_phone,
		        recipient_token, template_id, subject, body, metadata, idempotency_key, scheduled_at, created_at, expires_at,
		        COUNT(*) OVER() AS total_count
		 FROM notifications
		 WHERE tenant_id = $1
		   AND ($2 = '' OR channel = $2)
		   AND ($3 = '' OR status = $3)
		   AND ($4::timestamptz IS NULL OR created_at >= $4)
		   AND ($5::timestamptz IS NULL OR created_at <= $5)
		 ORDER BY created_at DESC
		 LIMIT $6 OFFSET $7`,
		tenantID, string(f.Channel), string(f.Status), f.From, f.To, f.Limit, f.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*domain.Notification
	var total int
	for rows.Next() {
		var n domain.Notification
		var channel, priority, status string
		var idempotencyKey *string
		if err := rows.Scan(
			&n.ID, &n.TenantID, &channel, &priority, &status, &n.RecipientID, &n.RecipientEmail, &n.RecipientPhone,
			&n.RecipientToken, &n.TemplateID, &n.Subject, &n.Body, &n.Metadata, &idempotencyKey, &n.ScheduledAt,
			&n.CreatedAt, &n.ExpiresAt, &total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		n.Channel = domain.Channel(channel)
		n.Priority = domain.Priority(priority)
		n.Status = domain.Status(status)
		if idempotencyKey != nil {
			n.IdempotencyKey = *idempotencyKey
		}
		notifications = append(notifications, &n)
	}
	return notifications, total, nil
}

// UpdateStatus sets the notification's top-level status. Called by channel consumers
// (Phase 6+) as a delivery attempt resolves.
func (r *NotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	tag, err := r.pool.Exec(ctx, `UPDATE notifications SET status = $1 WHERE id = $2`, string(status), id)
	if err != nil {
		return fmt.Errorf("update notification status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
