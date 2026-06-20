package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// TemplateRepositoryInterface is the seam used by handlers so they can be unit-tested
// against a mock instead of a real Postgres connection.
type TemplateRepositoryInterface interface {
	Create(ctx context.Context, tenantID uuid.UUID, name string, channel domain.Channel, subject, body string) (*domain.NotificationTemplate, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.NotificationTemplate, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.NotificationTemplate, error)
	Update(ctx context.Context, tenantID, id uuid.UUID, name string, channel domain.Channel, subject, body string) (*domain.NotificationTemplate, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// TemplateRepository handles all DB operations for the notification_templates table.
type TemplateRepository struct {
	pool Executor
}

var _ TemplateRepositoryInterface = (*TemplateRepository)(nil)

func NewTemplateRepository(pool Executor) *TemplateRepository {
	return &TemplateRepository{pool: pool}
}

func (r *TemplateRepository) Create(ctx context.Context, tenantID uuid.UUID, name string, channel domain.Channel, subject, body string) (*domain.NotificationTemplate, error) {
	var t domain.NotificationTemplate
	err := r.pool.QueryRow(ctx,
		`INSERT INTO notification_templates (tenant_id, name, channel, subject, body)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, tenant_id, name, channel, subject, body, created_at, updated_at`,
		tenantID, name, channel, subject, body,
	).Scan(&t.ID, &t.TenantID, &t.Name, &t.Channel, &t.Subject, &t.Body, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create template: %w", err)
	}
	return &t, nil
}

func (r *TemplateRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.NotificationTemplate, error) {
	var t domain.NotificationTemplate
	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, channel, subject, body, created_at, updated_at
		 FROM notification_templates WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&t.ID, &t.TenantID, &t.Name, &t.Channel, &t.Subject, &t.Body, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get template by id: %w", err)
	}
	return &t, nil
}

func (r *TemplateRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.NotificationTemplate, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, name, channel, subject, body, created_at, updated_at
		 FROM notification_templates WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []*domain.NotificationTemplate
	for rows.Next() {
		var t domain.NotificationTemplate
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.Channel, &t.Subject, &t.Body, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		templates = append(templates, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate templates: %w", err)
	}
	return templates, nil
}

func (r *TemplateRepository) Update(ctx context.Context, tenantID, id uuid.UUID, name string, channel domain.Channel, subject, body string) (*domain.NotificationTemplate, error) {
	var t domain.NotificationTemplate
	err := r.pool.QueryRow(ctx,
		`UPDATE notification_templates
		 SET name = $1, channel = $2, subject = $3, body = $4, updated_at = NOW()
		 WHERE id = $5 AND tenant_id = $6
		 RETURNING id, tenant_id, name, channel, subject, body, created_at, updated_at`,
		name, channel, subject, body, id, tenantID,
	).Scan(&t.ID, &t.TenantID, &t.Name, &t.Channel, &t.Subject, &t.Body, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("update template: %w", err)
	}
	return &t, nil
}

func (r *TemplateRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM notification_templates WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
