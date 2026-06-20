package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func templateCols() []string {
	return []string{"id", "tenant_id", "name", "channel", "subject", "body", "created_at", "updated_at"}
}

func TestTemplateRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(templateCols()).AddRow(id, tenantID, "welcome", string(domain.ChannelEmail), "Hi", "body", now, now)
	mock.ExpectQuery("INSERT INTO notification_templates").
		WithArgs(tenantID, "welcome", domain.ChannelEmail, "Hi", "body").
		WillReturnRows(rows)

	tmpl, err := repo.Create(context.Background(), tenantID, "welcome", domain.ChannelEmail, "Hi", "body")
	require.NoError(t, err)
	assert.Equal(t, id, tmpl.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Create_AlreadyExists(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("INSERT INTO notification_templates").
		WithArgs(tenantID, "welcome", domain.ChannelEmail, "Hi", "body").
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	_, err = repo.Create(context.Background(), tenantID, "welcome", domain.ChannelEmail, "Hi", "body")
	assert.ErrorIs(t, err, ErrAlreadyExists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Create_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("INSERT INTO notification_templates").
		WithArgs(tenantID, "welcome", domain.ChannelEmail, "Hi", "body").
		WillReturnError(errors.New("boom"))

	_, err = repo.Create(context.Background(), tenantID, "welcome", domain.ChannelEmail, "Hi", "body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create template")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(templateCols()).AddRow(id, tenantID, "welcome", string(domain.ChannelEmail), "Hi", "body", now, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM notification_templates WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnRows(rows)

	tmpl, err := repo.GetByID(context.Background(), tenantID, id)
	require.NoError(t, err)
	assert.Equal(t, id, tmpl.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM notification_templates WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), tenantID, id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_GetByID_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM notification_templates WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnError(errors.New("boom"))

	_, err = repo.GetByID(context.Background(), tenantID, id)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_ListByTenant_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(templateCols()).AddRow(id, tenantID, "welcome", domain.ChannelEmail, "Hi", "body", now, now)
	mock.ExpectQuery("SELECT (.|\n)*FROM notification_templates WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	templates, err := repo.ListByTenant(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_ListByTenant_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()

	rows := pgxmock.NewRows(templateCols())
	mock.ExpectQuery("SELECT (.|\n)*FROM notification_templates WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(rows)

	templates, err := repo.ListByTenant(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Empty(t, templates)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_ListByTenant_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT (.|\n)*FROM notification_templates WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnError(errors.New("boom"))

	_, err = repo.ListByTenant(context.Background(), tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list templates")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Update_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows(templateCols()).AddRow(id, tenantID, "updated", string(domain.ChannelEmail), "Hi2", "body2", now, now)
	mock.ExpectQuery("UPDATE notification_templates").
		WithArgs("updated", domain.ChannelEmail, "Hi2", "body2", id, tenantID).
		WillReturnRows(rows)

	tmpl, err := repo.Update(context.Background(), tenantID, id, "updated", domain.ChannelEmail, "Hi2", "body2")
	require.NoError(t, err)
	assert.Equal(t, "updated", tmpl.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Update_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectQuery("UPDATE notification_templates").
		WithArgs("updated", domain.ChannelEmail, "Hi2", "body2", id, tenantID).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.Update(context.Background(), tenantID, id, "updated", domain.ChannelEmail, "Hi2", "body2")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Update_AlreadyExists(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectQuery("UPDATE notification_templates").
		WithArgs("updated", domain.ChannelEmail, "Hi2", "body2", id, tenantID).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	_, err = repo.Update(context.Background(), tenantID, id, "updated", domain.ChannelEmail, "Hi2", "body2")
	assert.ErrorIs(t, err, ErrAlreadyExists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Update_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectQuery("UPDATE notification_templates").
		WithArgs("updated", domain.ChannelEmail, "Hi2", "body2", id, tenantID).
		WillReturnError(errors.New("boom"))

	_, err = repo.Update(context.Background(), tenantID, id, "updated", domain.ChannelEmail, "Hi2", "body2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update template")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Delete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectExec("DELETE FROM notification_templates WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(context.Background(), tenantID, id)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Delete_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectExec("DELETE FROM notification_templates WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.Delete(context.Background(), tenantID, id)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTemplateRepository_Delete_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTemplateRepository(mock)
	tenantID := uuid.New()
	id := uuid.New()

	mock.ExpectExec("DELETE FROM notification_templates WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnError(errors.New("boom"))

	err = repo.Delete(context.Background(), tenantID, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete template")
	assert.NoError(t, mock.ExpectationsWereMet())
}
