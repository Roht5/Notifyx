package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantChannelRepository_Upsert_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantChannelRepository(mock)
	tenantID := uuid.New()

	mock.ExpectExec("INSERT INTO tenant_channels").
		WithArgs(tenantID, string(domain.ChannelEmail), true).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Upsert(context.Background(), tenantID, domain.ChannelEmail, true)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantChannelRepository_Upsert_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantChannelRepository(mock)
	tenantID := uuid.New()

	mock.ExpectExec("INSERT INTO tenant_channels").
		WithArgs(tenantID, string(domain.ChannelEmail), true).
		WillReturnError(errors.New("boom"))

	err = repo.Upsert(context.Background(), tenantID, domain.ChannelEmail, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert tenant channel")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantChannelRepository_GetByTenantID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantChannelRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "channel", "enabled", "created_at"}).
		AddRow(id, tenantID, string(domain.ChannelEmail), true, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_channels WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	channels, err := repo.GetByTenantID(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, channels, 1)
	assert.True(t, channels[0].Enabled)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantChannelRepository_GetByTenantID_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantChannelRepository(mock)
	tenantID := uuid.New()

	rows := pgxmock.NewRows([]string{"id", "tenant_id", "channel", "enabled", "created_at"})
	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_channels WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	channels, err := repo.GetByTenantID(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Empty(t, channels)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantChannelRepository_GetByTenantID_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantChannelRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM tenant_channels WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnError(errors.New("boom"))

	_, err = repo.GetByTenantID(context.Background(), tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get tenant channels")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantChannelRepository_IsEnabled_True(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantChannelRepository(mock)
	tenantID := uuid.New()

	rows := pgxmock.NewRows([]string{"enabled"}).AddRow(true)
	mock.ExpectQuery("SELECT enabled FROM tenant_channels WHERE tenant_id = \\$1 AND channel = \\$2").
		WithArgs(tenantID, string(domain.ChannelEmail)).
		WillReturnRows(rows)

	enabled, err := repo.IsEnabled(context.Background(), tenantID, domain.ChannelEmail)
	require.NoError(t, err)
	assert.True(t, enabled)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTenantChannelRepository_IsEnabled_NoRowMeansFalse(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTenantChannelRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT enabled FROM tenant_channels WHERE tenant_id = \\$1 AND channel = \\$2").
		WithArgs(tenantID, string(domain.ChannelSMS)).
		WillReturnError(errors.New("no rows"))

	enabled, err := repo.IsEnabled(context.Background(), tenantID, domain.ChannelSMS)
	require.NoError(t, err) // implementation swallows the error and returns false, nil
	assert.False(t, enabled)
	assert.NoError(t, mock.ExpectationsWereMet())
}
