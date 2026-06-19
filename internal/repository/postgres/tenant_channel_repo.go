package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

type TenantChannelRepository struct {
	pool *pgxpool.Pool
}

func NewTenantChannelRepository(pool *pgxpool.Pool) *TenantChannelRepository {
	return &TenantChannelRepository{pool: pool}
}

// Upsert inserts or updates the enabled flag for a tenant+channel combination.
// ON CONFLICT uses the unique constraint (tenant_id, channel).
func (r *TenantChannelRepository) Upsert(ctx context.Context, tenantID uuid.UUID, channel domain.Channel, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tenant_channels (tenant_id, channel, enabled)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (tenant_id, channel) DO UPDATE SET enabled = EXCLUDED.enabled`,
		tenantID, string(channel), enabled,
	)
	if err != nil {
		return fmt.Errorf("upsert tenant channel: %w", err)
	}
	return nil
}

func (r *TenantChannelRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantChannel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, channel, enabled, created_at
		 FROM tenant_channels WHERE tenant_id = $1`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get tenant channels: %w", err)
	}
	defer rows.Close()

	var channels []*domain.TenantChannel
	for rows.Next() {
		var tc domain.TenantChannel
		var ch string
		if err := rows.Scan(&tc.ID, &tc.TenantID, &ch, &tc.Enabled, &tc.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant channel: %w", err)
		}
		tc.Channel = domain.Channel(ch)
		channels = append(channels, &tc)
	}
	return channels, nil
}

// IsEnabled returns true if the tenant has the given channel enabled.
func (r *TenantChannelRepository) IsEnabled(ctx context.Context, tenantID uuid.UUID, channel domain.Channel) (bool, error) {
	var enabled bool
	err := r.pool.QueryRow(ctx,
		`SELECT enabled FROM tenant_channels WHERE tenant_id = $1 AND channel = $2`,
		tenantID, string(channel),
	).Scan(&enabled)
	if err != nil {
		return false, nil // no row means not opted in
	}
	return enabled, nil
}
