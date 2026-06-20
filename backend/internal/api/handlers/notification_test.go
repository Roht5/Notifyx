package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// notificationHandlerDeps bundles all mocks for a NotificationHandler under test. Pool is
// left nil intentionally — Send/Schedule/Batch all eventually call createAndQueue, which
// calls postgres.WithTx(ctx, h.svc.Pool, ...) directly against the concrete *pgxpool.Pool
// type (no interface seam exists for it; see final report). So these tests can only
// exercise the pre-persistence logic: tenant/channel checks, validation, template
// resolution, rate limiting, and dedup — all of which run before createAndQueue is
// reached. Tests that would need to get past createAndQueue successfully are out of
// scope here and are called out in the report instead of being silently skipped.
type notificationHandlerDeps struct {
	notifications *mockNotificationRepo
	deliveries    *mockDeliveryRepo
	channels      *mockChannelRepo
	templates     *mockTemplateRepo
	scheduled     *mockScheduledRepo
	producer      *mockProducer
	rateLimiter   *mockRateLimiter
	dedup         *mockDedup
}

func newNotificationHandler() (*NotificationHandler, *notificationHandlerDeps) {
	d := &notificationHandlerDeps{
		notifications: &mockNotificationRepo{},
		deliveries:    &mockDeliveryRepo{},
		channels:      &mockChannelRepo{},
		templates:     &mockTemplateRepo{},
		scheduled:     &mockScheduledRepo{},
		producer:      &mockProducer{},
		rateLimiter:   &mockRateLimiter{},
		dedup:         &mockDedup{},
	}
	svc := &NotificationService{
		Notifications: d.notifications,
		Deliveries:    d.deliveries,
		Channels:      d.channels,
		Templates:     d.templates,
		Scheduled:     d.scheduled,
		Producer:      d.producer,
		RateLimiter:   d.rateLimiter,
		Dedup:         d.dedup,
		Pool:          nil,
	}
	return NewNotificationHandler(svc, newTestLogger()), d
}

// --- Send: validation / pre-persistence paths ---

func TestNotificationHandler_Send_BadBody(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{bad json`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_Send_InvalidChannel(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"carrier-pigeon","body":"hi","recipient_email":"a@b.com","subject":"s"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Send_MissingRecipientField(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","body":"hi","subject":"s"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Send_ChannelCheckError(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(false, assert.AnError)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","body":"hi","subject":"s","recipient_email":"a@b.com"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNotificationHandler_Send_ChannelNotEnabled(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(false, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","body":"hi","subject":"s","recipient_email":"a@b.com"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestNotificationHandler_Send_TemplateBadID(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(true, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","recipient_email":"a@b.com","template_id":"not-a-uuid"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_Send_TemplateNotFound(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	templateID := uuid.New()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(true, nil)
	d.templates.On("GetByID", mock.Anything, tenant.ID, templateID).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","recipient_email":"a@b.com","template_id":"`+templateID.String()+`"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNotificationHandler_Send_TemplateChannelMismatch(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	templateID := uuid.New()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(true, nil)
	tmpl := &domain.NotificationTemplate{ID: templateID, TenantID: tenant.ID, Channel: domain.ChannelSMS, Body: "hi {{name}}"}
	d.templates.On("GetByID", mock.Anything, tenant.ID, templateID).Return(tmpl, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","recipient_email":"a@b.com","template_id":"`+templateID.String()+`"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Send_TemplateResolveError(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	templateID := uuid.New()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(true, nil)
	d.templates.On("GetByID", mock.Anything, tenant.ID, templateID).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","recipient_email":"a@b.com","template_id":"`+templateID.String()+`"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNotificationHandler_Send_DedupHit(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	existingID := uuid.New()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(true, nil)
	d.dedup.On("Reserve", mock.Anything, "idem-key", mock.AnythingOfType("uuid.UUID")).Return(false, existingID, nil)
	existing := &domain.Notification{ID: existingID, TenantID: tenant.ID, Status: domain.StatusQueued}
	d.notifications.On("GetByID", mock.Anything, existingID).Return(existing, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","recipient_email":"a@b.com","subject":"s","body":"hi","idempotency_key":"idem-key"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code) // duplicate -> 200, not 201
}

func TestNotificationHandler_Send_DedupReserveError(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(true, nil)
	d.dedup.On("Reserve", mock.Anything, "idem-key", mock.AnythingOfType("uuid.UUID")).Return(false, uuid.Nil, assert.AnError)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/send", `{"channel":"email","recipient_email":"a@b.com","subject":"s","body":"hi","idempotency_key":"idem-key"}`, tenant)
	err := h.Send(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNotificationHandler_Send_RateLimited_NoPoolPanicsSkipped(t *testing.T) {
	// processSend checks the rate limiter (and short-circuits to StatusQueuedRateLimited)
	// before calling createAndQueue (which needs h.svc.Pool, nil in these tests). Since
	// createAndQueue is still reached even on the rate-limited path (to persist the
	// rate-limited notification), this path cannot be exercised end-to-end without Pool —
	// see the deps comment. Documented here rather than silently omitted.
	t.Skip("requires h.svc.Pool (createAndQueue) — no interface seam exists for *pgxpool.Pool; see report")
}

// --- Schedule: validation / pre-persistence paths ---

func TestNotificationHandler_Schedule_BadBody(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/schedule", `{bad`, tenant)
	err := h.Schedule(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_Schedule_MissingScheduledAt(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/schedule", `{"channel":"email","recipient_email":"a@b.com","subject":"s","body":"hi"}`, tenant)
	err := h.Schedule(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Schedule_PastScheduledAt(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/schedule", `{"channel":"email","recipient_email":"a@b.com","subject":"s","body":"hi","scheduled_at":"2000-01-01T00:00:00Z"}`, tenant)
	err := h.Schedule(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Schedule_InvalidScheduledAtFormat(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/schedule", `{"channel":"email","recipient_email":"a@b.com","subject":"s","body":"hi","scheduled_at":"not-a-date"}`, tenant)
	err := h.Schedule(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Schedule_ChannelNotEnabled(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(false, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/schedule", `{"channel":"email","recipient_email":"a@b.com","subject":"s","body":"hi","scheduled_at":"2099-01-01T00:00:00Z"}`, tenant)
	err := h.Schedule(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestNotificationHandler_Schedule_ChannelCheckError(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(false, assert.AnError)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/schedule", `{"channel":"email","recipient_email":"a@b.com","subject":"s","body":"hi","scheduled_at":"2099-01-01T00:00:00Z"}`, tenant)
	err := h.Schedule(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Batch: validation / pre-persistence paths ---

func TestNotificationHandler_Batch_BadBody(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/batch", `{bad`, tenant)
	err := h.Batch(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_Batch_NoRecipients(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/batch", `{"channel":"email","subject":"s","body":"hi","recipients":[]}`, tenant)
	err := h.Batch(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Batch_InvalidRecipientField(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/batch", `{"channel":"email","subject":"s","body":"hi","recipients":[{"recipient_email":""}]}`, tenant)
	err := h.Batch(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestNotificationHandler_Batch_ChannelNotEnabled(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.channels.On("IsEnabled", mock.Anything, tenant.ID, domain.ChannelEmail).Return(false, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/notifications/batch", `{"channel":"email","subject":"s","body":"hi","recipients":[{"recipient_email":"a@b.com"}]}`, tenant)
	err := h.Batch(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- History ---

func TestNotificationHandler_History_Success(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	want := []*domain.Notification{{ID: uuid.New(), TenantID: tenant.ID}}
	d.notifications.On("GetByTenantID", mock.Anything, tenant.ID, mock.AnythingOfType("postgres.NotificationFilter")).Return(want, 1, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/history", "", tenant)
	err := h.History(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNotificationHandler_History_InvalidChannel(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/history?channel=bogus", "", tenant)
	err := h.History(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_History_InvalidStatus(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/history?status=bogus", "", tenant)
	err := h.History(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_History_InvalidFrom(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/history?from=bogus", "", tenant)
	err := h.History(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_History_InvalidTo(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/history?to=bogus", "", tenant)
	err := h.History(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_History_RepoError(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()

	d.notifications.On("GetByTenantID", mock.Anything, tenant.ID, mock.AnythingOfType("postgres.NotificationFilter")).Return(nil, 0, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/history", "", tenant)
	err := h.History(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Get ---

func TestNotificationHandler_Get_Success(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	id := uuid.New()

	n := &domain.Notification{ID: id, TenantID: tenant.ID}
	d.notifications.On("GetByID", mock.Anything, id).Return(n, nil)
	d.deliveries.On("GetByNotificationID", mock.Anything, id).Return([]*domain.NotificationDelivery{}, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNotificationHandler_Get_BadID(t *testing.T) {
	h, _ := newNotificationHandler()
	tenant := testTenant()

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/bad", "", tenant)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNotificationHandler_Get_NotFound(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	id := uuid.New()

	d.notifications.On("GetByID", mock.Anything, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNotificationHandler_Get_RepoError(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	id := uuid.New()

	d.notifications.On("GetByID", mock.Anything, id).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNotificationHandler_Get_WrongTenant(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	otherTenantID := uuid.New()
	id := uuid.New()

	n := &domain.Notification{ID: id, TenantID: otherTenantID}
	d.notifications.On("GetByID", mock.Anything, id).Return(n, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNotificationHandler_Get_DeliveriesError(t *testing.T) {
	h, d := newNotificationHandler()
	tenant := testTenant()
	id := uuid.New()

	n := &domain.Notification{ID: id, TenantID: tenant.ID}
	d.notifications.On("GetByID", mock.Anything, id).Return(n, nil)
	d.deliveries.On("GetByNotificationID", mock.Anything, id).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/notifications/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
