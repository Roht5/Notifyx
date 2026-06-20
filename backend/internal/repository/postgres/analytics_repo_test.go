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

func TestAnalyticsRepository_Summary_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	rows := pgxmock.NewRows([]string{"total_sent", "total_delivered", "total_failed"}).AddRow(10, 7, 2)
	mock.ExpectQuery("SELECT(.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1").
		WithArgs(tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed)).
		WillReturnRows(rows)

	summary, err := repo.Summary(context.Background(), tenantID, from, to)
	require.NoError(t, err)
	assert.Equal(t, 10, summary.TotalSent)
	assert.Equal(t, 7, summary.TotalDelivered)
	assert.Equal(t, 2, summary.TotalFailed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_Summary_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	mock.ExpectQuery("SELECT(.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1").
		WithArgs(tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed)).
		WillReturnError(errors.New("boom"))

	_, err = repo.Summary(context.Background(), tenantID, from, to)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "analytics summary")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_ChannelBreakdown_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	rows := pgxmock.NewRows([]string{"channel", "sent", "delivered", "failed"}).
		AddRow(string(domain.ChannelEmail), 10, 8, 1)
	mock.ExpectQuery("SELECT(.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1(.|\n)*GROUP BY channel").
		WithArgs(tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed)).
		WillReturnRows(rows)

	breakdown, err := repo.ChannelBreakdown(context.Background(), tenantID, from, to)
	require.NoError(t, err)
	require.Len(t, breakdown, 1)
	assert.Equal(t, domain.ChannelEmail, breakdown[0].Channel)
	assert.InDelta(t, 0.8, breakdown[0].DeliveryRate, 0.0001)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_ChannelBreakdown_ZeroSentNoDivideByZero(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	rows := pgxmock.NewRows([]string{"channel", "sent", "delivered", "failed"}).
		AddRow(string(domain.ChannelSMS), 0, 0, 0)
	mock.ExpectQuery("SELECT(.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1(.|\n)*GROUP BY channel").
		WithArgs(tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed)).
		WillReturnRows(rows)

	breakdown, err := repo.ChannelBreakdown(context.Background(), tenantID, from, to)
	require.NoError(t, err)
	require.Len(t, breakdown, 1)
	assert.Equal(t, float64(0), breakdown[0].DeliveryRate)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_ChannelBreakdown_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	rows := pgxmock.NewRows([]string{"channel", "sent", "delivered", "failed"})
	mock.ExpectQuery("SELECT(.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1(.|\n)*GROUP BY channel").
		WithArgs(tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed)).
		WillReturnRows(rows)

	breakdown, err := repo.ChannelBreakdown(context.Background(), tenantID, from, to)
	require.NoError(t, err)
	assert.Empty(t, breakdown)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_ChannelBreakdown_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	mock.ExpectQuery("SELECT(.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1(.|\n)*GROUP BY channel").
		WithArgs(tenantID, from, to, string(domain.StatusDelivered), string(domain.StatusFailed)).
		WillReturnError(errors.New("boom"))

	_, err = repo.ChannelBreakdown(context.Background(), tenantID, from, to)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "analytics channel breakdown")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_DLQTrend_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-48 * time.Hour)
	to := time.Now()

	rows := pgxmock.NewRows([]string{"day", "count"}).
		AddRow(from, 0).
		AddRow(to, 3)
	mock.ExpectQuery("SELECT g.day(.|\n)*FROM generate_series(.|\n)*LEFT JOIN(.|\n)*ORDER BY g.day").
		WithArgs(tenantID, from, to).
		WillReturnRows(rows)

	trend, err := repo.DLQTrend(context.Background(), tenantID, from, to)
	require.NoError(t, err)
	assert.Len(t, trend, 2)
	assert.Equal(t, 3, trend[1].Count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_DLQTrend_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-48 * time.Hour)
	to := time.Now()

	rows := pgxmock.NewRows([]string{"day", "count"})
	mock.ExpectQuery("SELECT g.day(.|\n)*FROM generate_series(.|\n)*LEFT JOIN(.|\n)*ORDER BY g.day").
		WithArgs(tenantID, from, to).
		WillReturnRows(rows)

	trend, err := repo.DLQTrend(context.Background(), tenantID, from, to)
	require.NoError(t, err)
	assert.Empty(t, trend)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnalyticsRepository_DLQTrend_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewAnalyticsRepository(mock)
	tenantID := uuid.New()
	from := time.Now().Add(-48 * time.Hour)
	to := time.Now()

	mock.ExpectQuery("SELECT g.day(.|\n)*FROM generate_series(.|\n)*LEFT JOIN(.|\n)*ORDER BY g.day").
		WithArgs(tenantID, from, to).
		WillReturnError(errors.New("boom"))

	_, err = repo.DLQTrend(context.Background(), tenantID, from, to)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "analytics dlq trend")
	assert.NoError(t, mock.ExpectationsWereMet())
}
