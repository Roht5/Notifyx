package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// TenantRepositoryInterface is the seam used by handlers/ratelimit so they can be
// unit-tested against a mock instead of a real Postgres connection.
type TenantRepositoryInterface interface {
	Create(ctx context.Context, name string, globalRateCap int) (*domain.Tenant, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	GetAll(ctx context.Context) ([]*domain.Tenant, error)
	Update(ctx context.Context, id uuid.UUID, name string) (*domain.Tenant, error)
	UpdateGlobalRateCap(ctx context.Context, id uuid.UUID, globalRateCap int) (*domain.Tenant, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// TenantRepository handles all DB operations for the tenants table.
// pool is an Executor (not *pgxpool.Pool directly) so the same repository type
// can also be constructed against a transaction — see WithTx in db.go.
type TenantRepository struct {
	pool Executor
}

var _ TenantRepositoryInterface = (*TenantRepository)(nil)

func NewTenantRepository(pool Executor) *TenantRepository {
	return &TenantRepository{pool: pool}
}

// Create inserts a new tenant. globalRateCap seeds Tenant.GlobalRateCap explicitly
// (sourced from DEFAULT_GLOBAL_CAP by the caller) rather than relying on the column's
// SQL default, so that env var still controls new-tenant behavior without a migration.
func (r *TenantRepository) Create(ctx context.Context, name string, globalRateCap int) (*domain.Tenant, error) {
	var t domain.Tenant
	err := r.pool.QueryRow(ctx,
		`INSERT INTO tenants (name, global_rate_cap) VALUES ($1, $2) RETURNING id, name, global_rate_cap, created_at`,
		name, globalRateCap,
	).Scan(&t.ID, &t.Name, &t.GlobalRateCap, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	var t domain.Tenant
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, global_rate_cap, created_at FROM tenants WHERE id = $1`,
		id,
	).Scan(&t.ID, &t.Name, &t.GlobalRateCap, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get tenant by id: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) GetAll(ctx context.Context) ([]*domain.Tenant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, global_rate_cap, created_at FROM tenants ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []*domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.GlobalRateCap, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenants: %w", err)
	}
	return tenants, nil
}

func (r *TenantRepository) Update(ctx context.Context, id uuid.UUID, name string) (*domain.Tenant, error) {
	var t domain.Tenant
	err := r.pool.QueryRow(ctx,
		`UPDATE tenants SET name = $1 WHERE id = $2 RETURNING id, name, global_rate_cap, created_at`,
		name, id,
	).Scan(&t.ID, &t.Name, &t.GlobalRateCap, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update tenant: %w", err)
	}
	return &t, nil
}

// UpdateGlobalRateCap sets the tenant-wide sliding-window cap used across all channels.
func (r *TenantRepository) UpdateGlobalRateCap(ctx context.Context, id uuid.UUID, globalRateCap int) (*domain.Tenant, error) {
	var t domain.Tenant
	err := r.pool.QueryRow(ctx,
		`UPDATE tenants SET global_rate_cap = $1 WHERE id = $2 RETURNING id, name, global_rate_cap, created_at`,
		globalRateCap, id,
	).Scan(&t.ID, &t.Name, &t.GlobalRateCap, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update tenant global rate cap: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tenant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
