package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/dedup"
	"github.com/rohit-bagade/notifyx/internal/domain"
	kafkatypes "github.com/rohit-bagade/notifyx/internal/kafka"
	"github.com/rohit-bagade/notifyx/internal/kafka/producer"
	"github.com/rohit-bagade/notifyx/internal/ratelimit"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// NotificationService groups the repositories and infra clients the notification handler
// needs. Producer is nil when KAFKA_BOOTSTRAP_SERVERS isn't configured — sends still
// persist, but publishing is skipped (logged as a warning) until Kafka is wired up.
// RateLimiter and Dedup are nil when REDIS_URL isn't configured — sends then skip both
// checks entirely (always allowed, no dedup).
type NotificationService struct {
	Notifications *postgres.NotificationRepository
	Deliveries    *postgres.NotificationDeliveryRepository
	Channels      *postgres.TenantChannelRepository
	Producer      *producer.Producer
	RateLimiter   *ratelimit.Limiter
	Dedup         *dedup.Deduplicator
}

// NotificationHandler handles all /api/v1/notifications routes.
type NotificationHandler struct {
	svc *NotificationService
	log *logger.Logger
}

func NewNotificationHandler(svc *NotificationService, log *logger.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, log: log}
}

// --- request / response types ---

type sendNotificationRequest struct {
	Channel        string         `json:"channel"`
	Priority       string         `json:"priority,omitempty"`
	RecipientID    string         `json:"recipient_id,omitempty"`
	RecipientEmail string         `json:"recipient_email,omitempty"`
	RecipientPhone string         `json:"recipient_phone,omitempty"`
	RecipientToken string         `json:"recipient_token,omitempty"`
	Subject        string         `json:"subject,omitempty"`
	Body           string         `json:"body"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}

func (r *sendNotificationRequest) Validate() error {
	if !isValidChannel(r.Channel) {
		return fmt.Errorf("invalid channel %q: must be one of email, push, sms, inapp", r.Channel)
	}
	if r.Priority != "" && !isValidPriority(r.Priority) {
		return fmt.Errorf("invalid priority %q: must be one of critical, high, normal, low", r.Priority)
	}
	if r.Body == "" {
		return errors.New("body is required")
	}
	return validateRecipientFields(domain.Channel(r.Channel), r.RecipientEmail, r.RecipientPhone, r.RecipientToken, r.RecipientID, r.Subject)
}

type sendNotificationResponse struct {
	NotificationID string        `json:"notification_id"`
	Status         domain.Status `json:"status"`
}

type batchRecipient struct {
	RecipientID    string `json:"recipient_id,omitempty"`
	RecipientEmail string `json:"recipient_email,omitempty"`
	RecipientPhone string `json:"recipient_phone,omitempty"`
	RecipientToken string `json:"recipient_token,omitempty"`
}

type batchSendRequest struct {
	Channel        string           `json:"channel"`
	Priority       string           `json:"priority,omitempty"`
	Subject        string           `json:"subject,omitempty"`
	Body           string           `json:"body"`
	Metadata       map[string]any   `json:"metadata,omitempty"`
	Recipients     []batchRecipient `json:"recipients"`
	IdempotencyKey string           `json:"idempotency_key,omitempty"`
}

func (r *batchSendRequest) Validate() error {
	if !isValidChannel(r.Channel) {
		return fmt.Errorf("invalid channel %q: must be one of email, push, sms, inapp", r.Channel)
	}
	if r.Priority != "" && !isValidPriority(r.Priority) {
		return fmt.Errorf("invalid priority %q: must be one of critical, high, normal, low", r.Priority)
	}
	if r.Body == "" {
		return errors.New("body is required")
	}
	if len(r.Recipients) == 0 {
		return errors.New("recipients must not be empty")
	}
	channel := domain.Channel(r.Channel)
	for i, rec := range r.Recipients {
		if err := validateRecipientFields(channel, rec.RecipientEmail, rec.RecipientPhone, rec.RecipientToken, rec.RecipientID, r.Subject); err != nil {
			return fmt.Errorf("recipient %d: %w", i, err)
		}
	}
	return nil
}

type batchResult struct {
	Index          int    `json:"index"`
	NotificationID string `json:"notification_id,omitempty"`
	Error          string `json:"error,omitempty"`
}

type batchSendResponse struct {
	Results []batchResult `json:"results"`
}

type historyQueryParams struct {
	PaginationParams
	Channel string `query:"channel"`
	Status  string `query:"status"`
	From    string `query:"from"`
	To      string `query:"to"`
}

type notificationHistoryResponse struct {
	Notifications []*domain.Notification `json:"notifications"`
	Total         int                    `json:"total"`
	Page          int                    `json:"page"`
	Limit         int                    `json:"limit"`
}

type notificationDetailResponse struct {
	Notification *domain.Notification           `json:"notification"`
	Deliveries   []*domain.NotificationDelivery `json:"deliveries"`
}

// --- validation helpers ---

func isValidPriority(p string) bool {
	switch domain.Priority(p) {
	case domain.PriorityCritical, domain.PriorityHigh, domain.PriorityNormal, domain.PriorityLow:
		return true
	}
	return false
}

func isValidStatus(s string) bool {
	switch domain.Status(s) {
	case domain.StatusPending, domain.StatusQueued, domain.StatusQueuedRateLimited, domain.StatusDelivered, domain.StatusFailed:
		return true
	}
	return false
}

// validateRecipientFields checks that the recipient field required by channel is present,
// and for email, that a subject was also given.
func validateRecipientFields(channel domain.Channel, email, phone, token, recipientID, subject string) error {
	switch channel {
	case domain.ChannelEmail:
		if email == "" {
			return errors.New("recipient_email is required for channel email")
		}
		if subject == "" {
			return errors.New("subject is required for channel email")
		}
	case domain.ChannelPush:
		if token == "" {
			return errors.New("recipient_token is required for channel push")
		}
	case domain.ChannelSMS:
		if phone == "" {
			return errors.New("recipient_phone is required for channel sms")
		}
	case domain.ChannelInApp:
		if recipientID == "" {
			return errors.New("recipient_id is required for channel inapp")
		}
	}
	return nil
}

// --- handlers ---

// Send POST /api/v1/notifications/send
func (h *NotificationHandler) Send(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	var req sendNotificationRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return errResponse(c, http.StatusUnprocessableEntity, err.Error())
	}

	ctx := c.Request().Context()
	channel := domain.Channel(req.Channel)

	enabled, err := h.svc.Channels.IsEnabled(ctx, tenant.ID, channel)
	if err != nil {
		h.log.Errorw("check channel enabled failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to verify channel")
	}
	if !enabled {
		return errResponse(c, http.StatusForbidden, fmt.Sprintf("channel %q is not enabled for this tenant", req.Channel))
	}

	priority := domain.Priority(req.Priority)
	if priority == "" {
		priority = domain.PriorityNormal
	}

	notificationID, status, duplicate, errMsg := h.processSend(ctx, req.IdempotencyKey, postgres.CreateNotificationParams{
		ID:             uuid.New(),
		TenantID:       tenant.ID,
		Channel:        channel,
		Priority:       priority,
		RecipientID:    req.RecipientID,
		RecipientEmail: req.RecipientEmail,
		RecipientPhone: req.RecipientPhone,
		RecipientToken: req.RecipientToken,
		Subject:        req.Subject,
		Body:           req.Body,
		Metadata:       req.Metadata,
	})
	if errMsg != "" {
		return errResponse(c, http.StatusInternalServerError, errMsg)
	}

	httpStatus := http.StatusCreated
	if duplicate {
		httpStatus = http.StatusOK
	}
	return c.JSON(httpStatus, sendNotificationResponse{NotificationID: notificationID, Status: status})
}

// Batch POST /api/v1/notifications/batch
func (h *NotificationHandler) Batch(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	var req batchSendRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return errResponse(c, http.StatusUnprocessableEntity, err.Error())
	}

	ctx := c.Request().Context()
	channel := domain.Channel(req.Channel)

	enabled, err := h.svc.Channels.IsEnabled(ctx, tenant.ID, channel)
	if err != nil {
		h.log.Errorw("check channel enabled failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to verify channel")
	}
	if !enabled {
		return errResponse(c, http.StatusForbidden, fmt.Sprintf("channel %q is not enabled for this tenant", req.Channel))
	}

	priority := domain.Priority(req.Priority)
	if priority == "" {
		priority = domain.PriorityNormal
	}

	results := make([]batchResult, len(req.Recipients))
	for i, rec := range req.Recipients {
		// Combined with the recipient's position so one shared idempotency_key on the
		// batch request still dedups each recipient independently (architecture.md:
		// "Dedup check per recipient (idempotency_key + recipientId)").
		idempotencyKey := ""
		if req.IdempotencyKey != "" {
			idempotencyKey = fmt.Sprintf("%s:%d", req.IdempotencyKey, i)
		}

		notificationID, _, _, errMsg := h.processSend(ctx, idempotencyKey, postgres.CreateNotificationParams{
			ID:             uuid.New(),
			TenantID:       tenant.ID,
			Channel:        channel,
			Priority:       priority,
			RecipientID:    rec.RecipientID,
			RecipientEmail: rec.RecipientEmail,
			RecipientPhone: rec.RecipientPhone,
			RecipientToken: rec.RecipientToken,
			Subject:        req.Subject,
			Body:           req.Body,
			Metadata:       req.Metadata,
		})
		if errMsg != "" {
			results[i] = batchResult{Index: i, Error: errMsg}
			continue
		}
		results[i] = batchResult{Index: i, NotificationID: notificationID}
	}

	return c.JSON(http.StatusCreated, batchSendResponse{Results: results})
}

// History GET /api/v1/notifications/history
func (h *NotificationHandler) History(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	var q historyQueryParams
	if err := c.Bind(&q); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid query parameters")
	}
	q.Normalize()

	if q.Channel != "" && !isValidChannel(q.Channel) {
		return errResponse(c, http.StatusBadRequest, fmt.Sprintf("invalid channel %q", q.Channel))
	}
	if q.Status != "" && !isValidStatus(q.Status) {
		return errResponse(c, http.StatusBadRequest, fmt.Sprintf("invalid status %q", q.Status))
	}

	var from, to *time.Time
	if q.From != "" {
		t, err := time.Parse(time.RFC3339, q.From)
		if err != nil {
			return errResponse(c, http.StatusBadRequest, "from must be RFC3339 (e.g. 2026-06-01T00:00:00Z)")
		}
		from = &t
	}
	if q.To != "" {
		t, err := time.Parse(time.RFC3339, q.To)
		if err != nil {
			return errResponse(c, http.StatusBadRequest, "to must be RFC3339 (e.g. 2026-06-01T00:00:00Z)")
		}
		to = &t
	}

	notifications, total, err := h.svc.Notifications.GetByTenantID(c.Request().Context(), tenant.ID, postgres.NotificationFilter{
		Channel: domain.Channel(q.Channel),
		Status:  domain.Status(q.Status),
		From:    from,
		To:      to,
		Limit:   q.Limit,
		Offset:  q.Offset(),
	})
	if err != nil {
		h.log.Errorw("list notifications failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to list notifications")
	}

	return c.JSON(http.StatusOK, notificationHistoryResponse{
		Notifications: notifications,
		Total:         total,
		Page:          q.Page,
		Limit:         q.Limit,
	})
}

// Get GET /api/v1/notifications/:id
func (h *NotificationHandler) Get(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid notification id")
	}

	ctx := c.Request().Context()
	notification, err := h.svc.Notifications.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "notification not found")
		}
		h.log.Errorw("get notification failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to get notification")
	}
	// Notifications are tenant-scoped — a notification belonging to another tenant
	// must look identical to one that doesn't exist.
	if notification.TenantID != tenant.ID {
		return errResponse(c, http.StatusNotFound, "notification not found")
	}

	deliveries, err := h.svc.Deliveries.GetByNotificationID(ctx, id)
	if err != nil {
		h.log.Errorw("get deliveries failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to get delivery status")
	}

	return c.JSON(http.StatusOK, notificationDetailResponse{
		Notification: notification,
		Deliveries:   deliveries,
	})
}

// processSend runs the full per-notification pipeline shared by Send and Batch: dedup
// check, rate limit check, persist, and (if allowed) publish to Kafka. idempotencyKey may
// be empty to skip dedup entirely. p.ID must already be set by the caller — it's generated
// up front so a dedup reservation can point at it before the notification is created.
//
// Returns either a fresh or a deduped notification ID + status, or a non-empty errMsg
// (already safe to surface to the client) on failure.
func (h *NotificationHandler) processSend(ctx context.Context, idempotencyKey string, p postgres.CreateNotificationParams) (notificationID string, status domain.Status, duplicate bool, errMsg string) {
	if idempotencyKey != "" && h.svc.Dedup != nil {
		claimed, existingID, err := h.svc.Dedup.Reserve(ctx, idempotencyKey, p.ID)
		if err != nil {
			h.log.Errorw("dedup reserve failed", "error", err)
			return "", "", false, "failed to process request"
		}
		if !claimed {
			existing, err := h.svc.Notifications.GetByID(ctx, existingID)
			if err != nil {
				h.log.Errorw("dedup: fetch original notification failed", "error", err)
				return "", "", false, "failed to process request"
			}
			return existing.ID.String(), existing.Status, true, ""
		}
	}

	p.Status = domain.StatusPending
	publish := true
	if h.svc.RateLimiter != nil {
		allowed, err := h.svc.RateLimiter.Allow(ctx, p.TenantID, p.Channel)
		if err != nil {
			h.log.Errorw("rate limit check failed", "error", err)
			h.releaseDedup(ctx, idempotencyKey)
			return "", "", false, "failed to process request"
		}
		if !allowed {
			p.Status = domain.StatusQueuedRateLimited
			publish = false
		}
	}
	p.IdempotencyKey = idempotencyKey

	notification, _, err := h.createAndQueue(ctx, p, publish)
	if err != nil {
		h.log.Errorw("create and queue failed", "error", err)
		h.releaseDedup(ctx, idempotencyKey)
		return "", "", false, "failed to send notification"
	}

	return notification.ID.String(), notification.Status, false, ""
}

func (h *NotificationHandler) releaseDedup(ctx context.Context, idempotencyKey string) {
	if idempotencyKey == "" || h.svc.Dedup == nil {
		return
	}
	if err := h.svc.Dedup.Release(ctx, idempotencyKey); err != nil {
		h.log.Errorw("dedup release failed", "error", err)
	}
}

// createAndQueue persists a notification and its delivery row, then — if publish is true —
// publishes it to the channel's Kafka topic. If the producer isn't configured (no
// KAFKA_BOOTSTRAP_SERVERS), publishing is skipped and a warning is logged — the
// notification still persists so it isn't lost once Kafka is wired up.
func (h *NotificationHandler) createAndQueue(ctx context.Context, p postgres.CreateNotificationParams, publish bool) (*domain.Notification, *domain.NotificationDelivery, error) {
	notification, err := h.svc.Notifications.Create(ctx, p)
	if err != nil {
		return nil, nil, fmt.Errorf("create notification: %w", err)
	}

	delivery, err := h.svc.Deliveries.Create(ctx, notification.ID, p.Channel, p.Status)
	if err != nil {
		return nil, nil, fmt.Errorf("create delivery: %w", err)
	}

	if !publish {
		return notification, delivery, nil
	}

	if h.svc.Producer == nil {
		h.log.Warnw("kafka not configured — notification persisted but not queued for delivery",
			"notification_id", notification.ID, "channel", p.Channel)
		return notification, delivery, nil
	}

	msg := &kafkatypes.Message{
		NotificationID: notification.ID,
		DeliveryID:     delivery.ID,
		TenantID:       p.TenantID,
		Channel:        p.Channel,
		Priority:       p.Priority,
		RecipientID:    p.RecipientID,
		RecipientEmail: p.RecipientEmail,
		RecipientPhone: p.RecipientPhone,
		RecipientToken: p.RecipientToken,
		Subject:        p.Subject,
		Body:           p.Body,
		Metadata:       p.Metadata,
	}
	if err := h.svc.Producer.Publish(ctx, kafkatypes.TopicForChannel(p.Channel), msg); err != nil {
		return nil, nil, fmt.Errorf("publish to kafka: %w", err)
	}

	return notification, delivery, nil
}
