package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rohit-bagade/notifyx/internal/domain"
)

// APIKeyRepositoryInterface is the seam used by handlers/middleware so they can be
// unit-tested against a mock instead of a real Postgres connection.
type APIKeyRepositoryInterface interface {
	Create(ctx context.Context, tenantID uuid.UUID, keyHash string) (*domain.APIKey, error)
	GetTenantByKeyHash(ctx context.Context, keyHash string) (*domain.Tenant, error)
	GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.APIKey, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// APIKeyRepository handles storage and lookup of hashed API keys.
type APIKeyRepository struct {
	pool Executor
}

var _ APIKeyRepositoryInterface = (*APIKeyRepository)(nil)

func NewAPIKeyRepository(pool Executor) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

func (r *APIKeyRepository) Create(ctx context.Context, tenantID uuid.UUID, keyHash string) (*domain.APIKey, error) {
	var k domain.APIKey
	err := r.pool.QueryRow(ctx,
		`INSERT INTO api_keys (tenant_id, key_hash) VALUES ($1, $2)
		 RETURNING id, tenant_id, key_hash, created_at`,
		tenantID, keyHash,
	).Scan(&k.ID, &k.TenantID, &k.KeyHash, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}
	return &k, nil
}

// GetByKeyHash looks up the tenant that owns the given hashed key.
// This is called on every authenticated request — the hash must match exactly.
func (r *APIKeyRepository) GetTenantByKeyHash(ctx context.Context, keyHash string) (*domain.Tenant, error) {
	var t domain.Tenant
	err := r.pool.QueryRow(ctx,
		`SELECT t.id, t.name, t.created_at
		 FROM api_keys k
		 JOIN tenants t ON t.id = k.tenant_id
		 WHERE k.key_hash = $1`,
		keyHash,
	).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get tenant by key hash: %w", err)
	}
	return &t, nil
}

func (r *APIKeyRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.APIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, key_hash, created_at FROM api_keys WHERE tenant_id = $1`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		var k domain.APIKey
		if err := rows.Scan(&k.ID, &k.TenantID, &k.KeyHash, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, &k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

func (r *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
