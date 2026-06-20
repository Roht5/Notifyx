package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "key_hash", "created_at"}).
		AddRow(id, tenantID, "hash123", now)
	mock.ExpectQuery("INSERT INTO api_keys").
		WithArgs(tenantID, "hash123").
		WillReturnRows(rows)

	key, err := repo.Create(context.Background(), tenantID, "hash123")
	require.NoError(t, err)
	assert.Equal(t, id, key.ID)
	assert.Equal(t, "hash123", key.KeyHash)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_Create_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("INSERT INTO api_keys").
		WithArgs(tenantID, "hash123").
		WillReturnError(errors.New("boom"))

	_, err = repo.Create(context.Background(), tenantID, "hash123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create api key")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_GetTenantByKeyHash_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	tenantID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "name", "created_at"}).AddRow(tenantID, "acme", now)
	mock.ExpectQuery("SELECT (.|\n)*FROM api_keys k(.|\n)*JOIN tenants t(.|\n)*WHERE k.key_hash = \\$1").
		WithArgs("hash123").
		WillReturnRows(rows)

	tenant, err := repo.GetTenantByKeyHash(context.Background(), "hash123")
	require.NoError(t, err)
	assert.Equal(t, tenantID, tenant.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_GetTenantByKeyHash_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)

	mock.ExpectQuery("SELECT (.|\n)*FROM api_keys k(.|\n)*JOIN tenants t(.|\n)*WHERE k.key_hash = \\$1").
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetTenantByKeyHash(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_GetTenantByKeyHash_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)

	mock.ExpectQuery("SELECT (.|\n)*FROM api_keys k(.|\n)*JOIN tenants t(.|\n)*WHERE k.key_hash = \\$1").
		WithArgs("hash123").
		WillReturnError(errors.New("boom"))

	_, err = repo.GetTenantByKeyHash(context.Background(), "hash123")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_GetByTenantID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "key_hash", "created_at"}).
		AddRow(id, tenantID, "hash123", now)
	mock.ExpectQuery("SELECT (.|\n)*FROM api_keys WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	keys, err := repo.GetByTenantID(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_GetByTenantID_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	tenantID := uuid.New()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "key_hash", "created_at"})
	mock.ExpectQuery("SELECT (.|\n)*FROM api_keys WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	keys, err := repo.GetByTenantID(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Empty(t, keys)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_GetByTenantID_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM api_keys WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnError(errors.New("boom"))

	_, err = repo.GetByTenantID(context.Background(), tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list api keys")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_Delete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	id := uuid.New()

	mock.ExpectExec("DELETE FROM api_keys WHERE id = \\$1").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(context.Background(), id)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_Delete_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	id := uuid.New()

	mock.ExpectExec("DELETE FROM api_keys WHERE id = \\$1").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.Delete(context.Background(), id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyRepository_Delete_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAPIKeyRepository(mock)
	id := uuid.New()

	mock.ExpectExec("DELETE FROM api_keys WHERE id = \\$1").
		WithArgs(id).
		WillReturnError(errors.New("boom"))

	err = repo.Delete(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete api key")
	assert.NoError(t, mock.ExpectationsWereMet())
}
