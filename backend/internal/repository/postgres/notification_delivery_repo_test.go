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

func TestNotificationDeliveryRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	notificationID := uuid.New()
	deliveryID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "notification_id", "channel", "status", "attempts", "created_at"}).
		AddRow(deliveryID, notificationID, string(domain.ChannelEmail), string(domain.StatusQueued), 0, now)
	mock.ExpectQuery("INSERT INTO notification_deliveries").
		WithArgs(notificationID, string(domain.ChannelEmail), string(domain.StatusQueued)).
		WillReturnRows(rows)

	d, err := repo.Create(context.Background(), notificationID, domain.ChannelEmail, "")
	require.NoError(t, err)
	assert.Equal(t, deliveryID, d.ID)
	assert.Equal(t, domain.StatusQueued, d.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_Create_ExplicitStatus(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	notificationID := uuid.New()
	deliveryID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "notification_id", "channel", "status", "attempts", "created_at"}).
		AddRow(deliveryID, notificationID, string(domain.ChannelSMS), string(domain.StatusQueuedRateLimited), 0, now)
	mock.ExpectQuery("INSERT INTO notification_deliveries").
		WithArgs(notificationID, string(domain.ChannelSMS), string(domain.StatusQueuedRateLimited)).
		WillReturnRows(rows)

	d, err := repo.Create(context.Background(), notificationID, domain.ChannelSMS, domain.StatusQueuedRateLimited)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusQueuedRateLimited, d.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_Create_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	notificationID := uuid.New()

	mock.ExpectQuery("INSERT INTO notification_deliveries").
		WithArgs(notificationID, string(domain.ChannelEmail), string(domain.StatusQueued)).
		WillReturnError(errors.New("boom"))

	_, err = repo.Create(context.Background(), notificationID, domain.ChannelEmail, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create notification delivery")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_SetStatus_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notification_deliveries SET status = \\$1 WHERE id = \\$2").
		WithArgs(string(domain.StatusQueued), id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.SetStatus(context.Background(), id, domain.StatusQueued)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_SetStatus_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notification_deliveries SET status = \\$1 WHERE id = \\$2").
		WithArgs(string(domain.StatusQueued), id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = repo.SetStatus(context.Background(), id, domain.StatusQueued)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_SetStatus_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notification_deliveries SET status = \\$1 WHERE id = \\$2").
		WithArgs(string(domain.StatusQueued), id).
		WillReturnError(errors.New("boom"))

	err = repo.SetStatus(context.Background(), id, domain.StatusQueued)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set delivery status")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_UpdateStatus_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notification_deliveries(.|\n)*SET status = \\$1").
		WithArgs(string(domain.StatusDelivered), "", id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdateStatus(context.Background(), id, domain.StatusDelivered, "")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_UpdateStatus_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notification_deliveries(.|\n)*SET status = \\$1").
		WithArgs(string(domain.StatusFailed), "timeout", id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = repo.UpdateStatus(context.Background(), id, domain.StatusFailed, "timeout")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_UpdateStatus_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notification_deliveries(.|\n)*SET status = \\$1").
		WithArgs(string(domain.StatusFailed), "timeout", id).
		WillReturnError(errors.New("boom"))

	err = repo.UpdateStatus(context.Background(), id, domain.StatusFailed, "timeout")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update delivery status")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_GetByNotificationID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	notificationID := uuid.New()
	deliveryID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{"id", "notification_id", "channel", "status", "attempts", "error_message", "delivered_at", "created_at"}).
		AddRow(deliveryID, notificationID, string(domain.ChannelEmail), string(domain.StatusDelivered), 1, (*string)(nil), &now, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM notification_deliveries WHERE notification_id = \\$1").
		WithArgs(notificationID).
		WillReturnRows(rows)

	deliveries, err := repo.GetByNotificationID(context.Background(), notificationID)
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)
	assert.Equal(t, deliveryID, deliveries[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_GetByNotificationID_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	notificationID := uuid.New()

	rows := pgxmock.NewRows([]string{"id", "notification_id", "channel", "status", "attempts", "error_message", "delivered_at", "created_at"})
	mock.ExpectQuery("SELECT (.|\n)*FROM notification_deliveries WHERE notification_id = \\$1").
		WithArgs(notificationID).
		WillReturnRows(rows)

	deliveries, err := repo.GetByNotificationID(context.Background(), notificationID)
	require.NoError(t, err)
	assert.Empty(t, deliveries)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationDeliveryRepository_GetByNotificationID_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationDeliveryRepository(mock)
	notificationID := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM notification_deliveries WHERE notification_id = \\$1").
		WithArgs(notificationID).
		WillReturnError(errors.New("boom"))

	_, err = repo.GetByNotificationID(context.Background(), notificationID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get deliveries by notification id")
	assert.NoError(t, mock.ExpectationsWereMet())
}
