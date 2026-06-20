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

func notificationCols() []string {
	return []string{
		"id", "tenant_id", "channel", "priority", "status", "recipient_id", "recipient_email",
		"recipient_phone", "recipient_token", "template_id", "subject", "body", "metadata",
		"idempotency_key", "scheduled_at", "created_at", "expires_at",
	}
}

func sampleNotificationRow(id, tenantID uuid.UUID) []any {
	var templateID *uuid.UUID
	var scheduledAt *time.Time
	return []any{
		id, tenantID, string(domain.ChannelEmail), string(domain.PriorityNormal), string(domain.StatusPending),
		"rec1", "rec@example.com", "", "", templateID, "subj", "body", map[string]any{}, nil, scheduledAt,
		time.Now(), time.Now().Add(24 * time.Hour),
	}
}

func TestNotificationRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()

	rows := pgxmock.NewRows(notificationCols()).AddRow(sampleNotificationRow(id, tenantID)...)
	var templateID *uuid.UUID
	var scheduledAt *time.Time
	mock.ExpectQuery("INSERT INTO notifications").
		WithArgs(id, tenantID, string(domain.ChannelEmail), string(domain.PriorityNormal), string(domain.StatusPending),
			"rec1", "rec@example.com", "", "", templateID, "subj", "body", map[string]any{}, (*string)(nil), scheduledAt).
		WillReturnRows(rows)

	n, err := repo.Create(context.Background(), CreateNotificationParams{
		ID: id, TenantID: tenantID, Channel: domain.ChannelEmail, Priority: domain.PriorityNormal,
		RecipientID: "rec1", RecipientEmail: "rec@example.com", Subject: "subj", Body: "body",
	})
	require.NoError(t, err)
	assert.Equal(t, id, n.ID)
	assert.Equal(t, domain.ChannelEmail, n.Channel)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_Create_DefaultsStatusAndMetadata(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()

	rows := pgxmock.NewRows(notificationCols()).AddRow(sampleNotificationRow(id, tenantID)...)
	var templateID *uuid.UUID
	var scheduledAt *time.Time
	mock.ExpectQuery("INSERT INTO notifications").
		WithArgs(id, tenantID, string(domain.ChannelEmail), string(domain.PriorityNormal), string(domain.StatusPending),
			"", "", "", "", templateID, "", "", map[string]any{}, (*string)(nil), scheduledAt).
		WillReturnRows(rows)

	_, err = repo.Create(context.Background(), CreateNotificationParams{
		ID: id, TenantID: tenantID, Channel: domain.ChannelEmail, Priority: domain.PriorityNormal,
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_Create_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()

	mock.ExpectQuery("INSERT INTO notifications").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("boom"))

	_, err = repo.Create(context.Background(), CreateNotificationParams{
		ID: id, TenantID: tenantID, Channel: domain.ChannelEmail, Priority: domain.PriorityNormal,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create notification")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()

	rows := pgxmock.NewRows(notificationCols()).AddRow(sampleNotificationRow(id, tenantID)...)
	mock.ExpectQuery("SELECT (.|\n)*FROM notifications WHERE id = \\$1").WithArgs(id).WillReturnRows(rows)

	n, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, n.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM notifications WHERE id = \\$1").WithArgs(id).WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByID_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM notifications WHERE id = \\$1").WithArgs(id).WillReturnError(errors.New("conn lost"))

	_, err = repo.GetByID(context.Background(), id)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByTenantID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	tenantID := uuid.New()
	id1 := uuid.New()

	cols := append(notificationCols(), "total_count")
	row := append(sampleNotificationRow(id1, tenantID), 5)
	rows := pgxmock.NewRows(cols).AddRow(row...)

	mock.ExpectQuery("SELECT (.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1").
		WithArgs(tenantID, string(domain.ChannelEmail), "", (*time.Time)(nil), (*time.Time)(nil), 10, 0, "").
		WillReturnRows(rows)

	notifications, total, err := repo.GetByTenantID(context.Background(), tenantID, NotificationFilter{
		Channel: domain.ChannelEmail, Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, notifications, 1)
	assert.Equal(t, 5, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByTenantID_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	tenantID := uuid.New()

	cols := append(notificationCols(), "total_count")
	rows := pgxmock.NewRows(cols)

	mock.ExpectQuery("SELECT (.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1").
		WithArgs(tenantID, "", "", (*time.Time)(nil), (*time.Time)(nil), 10, 0, "").
		WillReturnRows(rows)

	notifications, total, err := repo.GetByTenantID(context.Background(), tenantID, NotificationFilter{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, notifications)
	assert.Equal(t, 0, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByTenantID_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	tenantID := uuid.New()

	var from, to *time.Time
	mock.ExpectQuery("SELECT (.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1").
		WithArgs(tenantID, "", "", from, to, 10, 0, "").
		WillReturnError(errors.New("boom"))

	_, _, err = repo.GetByTenantID(context.Background(), tenantID, NotificationFilter{Limit: 10})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list notifications")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByTenantID_WithRecipientFilter(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	tenantID := uuid.New()

	cols := append(notificationCols(), "total_count")
	rows := pgxmock.NewRows(cols)

	mock.ExpectQuery("SELECT (.|\n)*FROM notifications(.|\n)*WHERE tenant_id = \\$1").
		WithArgs(tenantID, "", string(domain.StatusDelivered), (*time.Time)(nil), (*time.Time)(nil), 5, 2, "someone@example.com").
		WillReturnRows(rows)

	_, _, err = repo.GetByTenantID(context.Background(), tenantID, NotificationFilter{
		Status: domain.StatusDelivered, Limit: 5, Offset: 2, Recipient: "someone@example.com",
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByStatus_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	tenantID := uuid.New()
	id1 := uuid.New()

	rows := pgxmock.NewRows(notificationCols()).AddRow(sampleNotificationRow(id1, tenantID)...)
	mock.ExpectQuery("SELECT (.|\n)*FROM notifications WHERE status = \\$1").
		WithArgs(string(domain.StatusQueuedRateLimited), 50).
		WillReturnRows(rows)

	res, err := repo.GetByStatus(context.Background(), domain.StatusQueuedRateLimited, 50)
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_GetByStatus_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)

	mock.ExpectQuery("SELECT (.|\n)*FROM notifications WHERE status = \\$1").
		WithArgs(string(domain.StatusPending), 10).
		WillReturnError(errors.New("boom"))

	_, err = repo.GetByStatus(context.Background(), domain.StatusPending, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get notifications by status")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_UpdateStatus_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notifications SET status = \\$1 WHERE id = \\$2").
		WithArgs(string(domain.StatusDelivered), id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdateStatus(context.Background(), id, domain.StatusDelivered)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_UpdateStatus_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notifications SET status = \\$1 WHERE id = \\$2").
		WithArgs(string(domain.StatusDelivered), id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = repo.UpdateStatus(context.Background(), id, domain.StatusDelivered)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_UpdateStatus_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	id := uuid.New()

	mock.ExpectExec("UPDATE notifications SET status = \\$1 WHERE id = \\$2").
		WithArgs(string(domain.StatusDelivered), id).
		WillReturnError(errors.New("boom"))

	err = repo.UpdateStatus(context.Background(), id, domain.StatusDelivered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update notification status")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_DeleteExpired_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	now := time.Now()

	mock.ExpectExec("DELETE FROM notifications WHERE expires_at < \\$1").
		WithArgs(now).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	n, err := repo.DeleteExpired(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_DeleteExpired_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewNotificationRepository(mock)
	now := time.Now()

	mock.ExpectExec("DELETE FROM notifications WHERE expires_at < \\$1").
		WithArgs(now).
		WillReturnError(errors.New("boom"))

	_, err = repo.DeleteExpired(context.Background(), now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete expired notifications")
	assert.NoError(t, mock.ExpectationsWereMet())
}
