package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduledNotificationRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	notificationID := uuid.New()
	id := uuid.New()
	scheduledAt := time.Now().Add(time.Hour)
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "notification_id", "scheduled_at", "fired", "created_at"}).
		AddRow(id, notificationID, scheduledAt, false, now)
	mock.ExpectQuery("INSERT INTO scheduled_notifications").
		WithArgs(notificationID, scheduledAt).
		WillReturnRows(rows)

	s, err := repo.Create(context.Background(), notificationID, scheduledAt)
	require.NoError(t, err)
	assert.Equal(t, id, s.ID)
	assert.False(t, s.Fired)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledNotificationRepository_Create_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	notificationID := uuid.New()
	scheduledAt := time.Now().Add(time.Hour)

	mock.ExpectQuery("INSERT INTO scheduled_notifications").
		WithArgs(notificationID, scheduledAt).
		WillReturnError(errors.New("boom"))

	_, err = repo.Create(context.Background(), notificationID, scheduledAt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create scheduled notification")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledNotificationRepository_GetDueUnfired_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	notificationID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "notification_id", "scheduled_at", "fired", "created_at"}).
		AddRow(id, notificationID, now, false, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM scheduled_notifications(.|\n)*WHERE fired = FALSE AND scheduled_at <= \\$1").
		WithArgs(now, 10).
		WillReturnRows(rows)

	due, err := repo.GetDueUnfired(context.Background(), now, 10)
	require.NoError(t, err)
	assert.Len(t, due, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledNotificationRepository_GetDueUnfired_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "notification_id", "scheduled_at", "fired", "created_at"})
	mock.ExpectQuery("SELECT (.|\n)*FROM scheduled_notifications(.|\n)*WHERE fired = FALSE AND scheduled_at <= \\$1").
		WithArgs(now, 10).
		WillReturnRows(rows)

	due, err := repo.GetDueUnfired(context.Background(), now, 10)
	require.NoError(t, err)
	assert.Empty(t, due)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledNotificationRepository_GetDueUnfired_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	now := time.Now()

	mock.ExpectQuery("SELECT (.|\n)*FROM scheduled_notifications(.|\n)*WHERE fired = FALSE AND scheduled_at <= \\$1").
		WithArgs(now, 10).
		WillReturnError(errors.New("boom"))

	_, err = repo.GetDueUnfired(context.Background(), now, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get due scheduled notifications")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledNotificationRepository_MarkFired_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE scheduled_notifications SET fired = TRUE WHERE id = \\$1").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.MarkFired(context.Background(), id)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledNotificationRepository_MarkFired_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE scheduled_notifications SET fired = TRUE WHERE id = \\$1").
		WithArgs(id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = repo.MarkFired(context.Background(), id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledNotificationRepository_MarkFired_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewScheduledNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE scheduled_notifications SET fired = TRUE WHERE id = \\$1").
		WithArgs(id).
		WillReturnError(errors.New("boom"))

	err = repo.MarkFired(context.Background(), id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mark scheduled notification fired")
	assert.NoError(t, mock.ExpectationsWereMet())
}
