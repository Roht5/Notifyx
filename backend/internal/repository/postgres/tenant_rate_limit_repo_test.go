package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantRateLimitRepository_Upsert_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()

	mock.ExpectExec("INSERT INTO tenant_rate_limits").
		WithArgs(tenantID, string(domain.ChannelEmail), 60).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Upsert(context.Background(), tenantID, domain.ChannelEmail, 60)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRateLimitRepository_Upsert_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()

	mock.ExpectExec("INSERT INTO tenant_rate_limits").
		WithArgs(tenantID, string(domain.ChannelEmail), 60).
		WillReturnError(errors.New("boom"))

	err = repo.Upsert(context.Background(), tenantID, domain.ChannelEmail, 60)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert tenant rate limit")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRateLimitRepository_GetByTenantID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "channel", "max_per_min", "created_at"}).
		AddRow(id, tenantID, string(domain.ChannelEmail), 60, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_rate_limits WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	limits, err := repo.GetByTenantID(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, limits, 1)
	assert.Equal(t, 60, limits[0].MaxPerMin)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRateLimitRepository_GetByTenantID_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "channel", "max_per_min", "created_at"})
	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_rate_limits WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	limits, err := repo.GetByTenantID(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Empty(t, limits)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRateLimitRepository_GetByTenantID_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_rate_limits WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnError(errors.New("boom"))

	_, err = repo.GetByTenantID(context.Background(), tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get rate limits")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRateLimitRepository_GetByChannel_Found(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "channel", "max_per_min", "created_at"}).
		AddRow(id, tenantID, string(domain.ChannelEmail), 60, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_rate_limits WHERE tenant_id = \\$1 AND channel = \\$2").
		WithArgs(tenantID, string(domain.ChannelEmail)).
		WillReturnRows(rows)

	rl, err := repo.GetByChannel(context.Background(), tenantID, domain.ChannelEmail)
	require.NoError(t, err)
	require.NotNil(t, rl)
	assert.Equal(t, 60, rl.MaxPerMin)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRateLimitRepository_GetByChannel_NotSetReturnsNilNil(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_rate_limits WHERE tenant_id = \\$1 AND channel = \\$2").
		WithArgs(tenantID, string(domain.ChannelSMS)).
		WillReturnError(pgx.ErrNoRows)

	rl, err := repo.GetByChannel(context.Background(), tenantID, domain.ChannelSMS)
	require.NoError(t, err) // no row means "use default", not an error
	assert.Nil(t, rl)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantRateLimitRepository_GetByChannel_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantRateLimitRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_rate_limits WHERE tenant_id = \\$1 AND channel = \\$2").
		WithArgs(tenantID, string(domain.ChannelSMS)).
		WillReturnError(errors.New("connection reset"))

	_, err = repo.GetByChannel(context.Background(), tenantID, domain.ChannelSMS)
	require.Error(t, err) // a real DB error must propagate, not be swallowed as "no override"
	assert.NoError(t, mock.ExpectationsWereMet())
}
