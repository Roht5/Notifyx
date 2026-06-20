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

func tenantCols() []string {
	return []string{"id", "name", "global_rate_cap", "created_at"}
}

func TestTenantRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(tenantCols()).AddRow(id, "acme", 100, now)
	mock.ExpectQuery("INSERT INTO tenants").
		WithArgs("acme", 100).
		WillReturnRows(rows)

	tenant, err := repo.Create(context.Background(), "acme", 100)
	require.NoError(t, err)
	assert.Equal(t, id, tenant.ID)
	assert.Equal(t, 100, tenant.GlobalRateCap)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_Create_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)

	mock.ExpectQuery("INSERT INTO tenants").
		WithArgs("acme", 100).
		WillReturnError(errors.New("boom"))

	_, err = repo.Create(context.Background(), "acme", 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create tenant")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(tenantCols()).AddRow(id, "acme", 100, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM tenants WHERE id = \\$1").WithArgs(id).WillReturnRows(rows)

	tenant, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, tenant.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM tenants WHERE id = \\$1").WithArgs(id).WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_GetByID_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM tenants WHERE id = \\$1").WithArgs(id).WillReturnError(errors.New("boom"))

	_, err = repo.GetByID(context.Background(), id)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_GetAll_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id1, id2 := uuid.New(), uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(tenantCols()).
		AddRow(id1, "acme", 100, now).
		AddRow(id2, "globex", 200, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM tenants ORDER BY created_at DESC").WillReturnRows(rows)

	tenants, err := repo.GetAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, tenants, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_GetAll_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)

	rows := pgxmock.NewRows(tenantCols())
	mock.ExpectQuery("SELECT (.|\n)*FROM tenants ORDER BY created_at DESC").WillReturnRows(rows)

	tenants, err := repo.GetAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tenants)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_GetAll_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)

	mock.ExpectQuery("SELECT (.|\n)*FROM tenants ORDER BY created_at DESC").WillReturnError(errors.New("boom"))

	_, err = repo.GetAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list tenants")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_Update_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(tenantCols()).AddRow(id, "newname", 100, now)
	mock.ExpectQuery("UPDATE tenants SET name = \\$1 WHERE id = \\$2").
		WithArgs("newname", id).
		WillReturnRows(rows)

	tenant, err := repo.Update(context.Background(), id, "newname")
	require.NoError(t, err)
	assert.Equal(t, "newname", tenant.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_Update_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("UPDATE tenants SET name = \\$1 WHERE id = \\$2").
		WithArgs("newname", id).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.Update(context.Background(), id, "newname")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_Update_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("UPDATE tenants SET name = \\$1 WHERE id = \\$2").
		WithArgs("newname", id).
		WillReturnError(errors.New("boom"))

	_, err = repo.Update(context.Background(), id, "newname")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update tenant")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_UpdateGlobalRateCap_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(tenantCols()).AddRow(id, "acme", 500, now)
	mock.ExpectQuery("UPDATE tenants SET global_rate_cap = \\$1 WHERE id = \\$2").
		WithArgs(500, id).
		WillReturnRows(rows)

	tenant, err := repo.UpdateGlobalRateCap(context.Background(), id, 500)
	require.NoError(t, err)
	assert.Equal(t, 500, tenant.GlobalRateCap)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_UpdateGlobalRateCap_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("UPDATE tenants SET global_rate_cap = \\$1 WHERE id = \\$2").
		WithArgs(500, id).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.UpdateGlobalRateCap(context.Background(), id, 500)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_UpdateGlobalRateCap_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("UPDATE tenants SET global_rate_cap = \\$1 WHERE id = \\$2").
		WithArgs(500, id).
		WillReturnError(errors.New("boom"))

	_, err = repo.UpdateGlobalRateCap(context.Background(), id, 500)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update tenant global rate cap")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_Delete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectExec("DELETE FROM tenants WHERE id = \\$1").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(context.Background(), id)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_Delete_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectExec("DELETE FROM tenants WHERE id = \\$1").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.Delete(context.Background(), id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRepository_Delete_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRepository(mock)
	id := uuid.New()

	mock.ExpectExec("DELETE FROM tenants WHERE id = \\$1").
		WithArgs(id).
		WillReturnError(errors.New("boom"))

	err = repo.Delete(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete tenant")
	assert.NoError(t, mock.ExpectationsWereMet())
}
