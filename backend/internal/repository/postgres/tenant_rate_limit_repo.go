package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

type TenantRateLimitRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRateLimitRepository(pool *pgxpool.Pool) *TenantRateLimitRepository {
	return &TenantRateLimitRepository{pool: pool}
}

func (r *TenantRateLimitRepository) Upsert(ctx context.Context, tenantID uuid.UUID, channel domain.Channel, maxPerMin int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tenant_rate_limits (tenant_id, channel, max_per_min)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (tenant_id, channel) DO UPDATE
		     SET max_per_min = EXCLUDED.max_per_min`,
		tenantID, string(channel), maxPerMin,
	)
	if err != nil {
		return fmt.Errorf("upsert tenant rate limit: %w", err)
	}
	return nil
}

func (r *TenantRateLimitRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantRateLimit, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, channel, max_per_min, created_at
		 FROM tenant_rate_limits WHERE tenant_id = $1`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get rate limits: %w", err)
	}
	defer rows.Close()

	var limits []*domain.TenantRateLimit
	for rows.Next() {
		var rl domain.TenantRateLimit
		var ch string
		if err := rows.Scan(&rl.ID, &rl.TenantID, &ch, &rl.MaxPerMin, &rl.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan rate limit: %w", err)
		}
		rl.Channel = domain.Channel(ch)
		limits = append(limits, &rl)
	}
	return limits, nil
}

// GetByChannel returns the rate limit config for a specific channel, or nil if not set.
func (r *TenantRateLimitRepository) GetByChannel(ctx context.Context, tenantID uuid.UUID, channel domain.Channel) (*domain.TenantRateLimit, error) {
	var rl domain.TenantRateLimit
	var ch string
	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, channel, max_per_min, created_at
		 FROM tenant_rate_limits WHERE tenant_id = $1 AND channel = $2`,
		tenantID, string(channel),
	).Scan(&rl.ID, &rl.TenantID, &ch, &rl.MaxPerMin, &rl.CreatedAt)
	if err != nil {
		return nil, nil // nil means "use default"
	}
	rl.Channel = domain.Channel(ch)
	return &rl, nil
}
