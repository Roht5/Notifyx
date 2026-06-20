package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testLogger(t *testing.T) *logger.Logger {
	log, err := logger.New("test", "error")
	require.NoError(t, err)
	return log
}

func newTestReplayer(t *testing.T) (*Replayer, *mockNotificationRepo, *mockDeliveryRepo, *mockTenantRepo, *mockRateLimitRepo, *mockProducer) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	tenants := &mockTenantRepo{}
	rateLimits := &mockRateLimitRepo{}
	limiter := New(client, tenants, rateLimits, 100)

	notifications := &mockNotificationRepo{}
	deliveries := &mockDeliveryRepo{}
	prod := &mockProducer{}

	r := NewReplayer(notifications, deliveries, limiter, prod, testLogger(t))
	return r, notifications, deliveries, tenants, rateLimits, prod
}

func sampleNotification(tenantID uuid.UUID) *domain.Notification {
	return &domain.Notification{
		ID:             uuid.New(),
		TenantID:       tenantID,
		Channel:        domain.ChannelEmail,
		Priority:       domain.PriorityNormal,
		Status:         domain.StatusQueuedRateLimited,
		RecipientEmail: "user@example.com",
		Body:           "hello",
	}
}

func TestReplayOnce_AllowedNotification_PublishesAndUpdatesStatus(t *testing.T) {
	r, notifications, deliveries, tenants, rateLimits, prod := newTestReplayer(t)
	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusQueuedRateLimited}

	notifications.On("GetByStatus", ctx, domain.StatusQueuedRateLimited, batchSize).Return([]*domain.Notification{n}, nil)
	tenants.On("GetByID", mock.Anything, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", mock.Anything, tenantID, domain.ChannelEmail).Return(nil, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, n.ID, domain.StatusQueued).Return(nil)
	deliveries.On("SetStatus", mock.Anything, delivery.ID, domain.StatusQueued).Return(nil)

	r.replayOnce(ctx)

	notifications.AssertExpectations(t)
	deliveries.AssertExpectations(t)
	prod.AssertExpectations(t)
}

func TestReplayOnce_StillRateLimited_DoesNotPublish(t *testing.T) {
	r, notifications, deliveries, tenants, rateLimits, prod := newTestReplayer(t)
	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)

	notifications.On("GetByStatus", ctx, domain.StatusQueuedRateLimited, batchSize).Return([]*domain.Notification{n}, nil)
	tenants.On("GetByID", mock.Anything, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 0}, nil)
	rateLimits.On("GetByChannel", mock.Anything, tenantID, domain.ChannelEmail).Return(nil, nil)

	r.replayOnce(ctx)

	deliveries.AssertNotCalled(t, "GetByNotificationID", mock.Anything, mock.Anything)
	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
	notifications.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestReplayOnce_NilProducer_SkipsPublishButUpdatesStatus(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	tenants := &mockTenantRepo{}
	rateLimits := &mockRateLimitRepo{}
	limiter := New(client, tenants, rateLimits, 100)

	notifications := &mockNotificationRepo{}
	deliveries := &mockDeliveryRepo{}

	r := NewReplayer(notifications, deliveries, limiter, nil, testLogger(t))

	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusQueuedRateLimited}

	notifications.On("GetByStatus", ctx, domain.StatusQueuedRateLimited, batchSize).Return([]*domain.Notification{n}, nil)
	tenants.On("GetByID", mock.Anything, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", mock.Anything, tenantID, domain.ChannelEmail).Return(nil, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	notifications.On("UpdateStatus", mock.Anything, n.ID, domain.StatusQueued).Return(nil)
	deliveries.On("SetStatus", mock.Anything, delivery.ID, domain.StatusQueued).Return(nil)

	r.replayOnce(ctx)

	notifications.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestReplayOnce_GetByStatusError_LogsAndReturns(t *testing.T) {
	r, notifications, _, _, _, prod := newTestReplayer(t)
	ctx := context.Background()

	notifications.On("GetByStatus", ctx, domain.StatusQueuedRateLimited, batchSize).Return(nil, assertErr)

	r.replayOnce(ctx)

	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestReplayOne_LimiterError_DoesNotPublish(t *testing.T) {
	r, _, deliveries, tenants, _, prod := newTestReplayer(t)
	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)

	tenants.On("GetByID", mock.Anything, tenantID).Return(nil, assertErr)

	r.replayOne(ctx, n)

	deliveries.AssertNotCalled(t, "GetByNotificationID", mock.Anything, mock.Anything)
	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestReplayOne_MissingDeliveryRow_DoesNotPublish(t *testing.T) {
	r, _, deliveries, tenants, rateLimits, prod := newTestReplayer(t)
	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)

	tenants.On("GetByID", mock.Anything, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", mock.Anything, tenantID, domain.ChannelEmail).Return(nil, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return(nil, nil)

	r.replayOne(ctx, n)

	prod.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestReplayOne_PublishError_DoesNotUpdateStatus(t *testing.T) {
	r, notifications, deliveries, tenants, rateLimits, prod := newTestReplayer(t)
	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusQueuedRateLimited}

	tenants.On("GetByID", mock.Anything, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", mock.Anything, tenantID, domain.ChannelEmail).Return(nil, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(assertErr)

	r.replayOne(ctx, n)

	notifications.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
	deliveries.AssertNotCalled(t, "SetStatus", mock.Anything, mock.Anything, mock.Anything)
}

func TestReplayOne_UpdateStatusError_StillCallsSetStatus(t *testing.T) {
	r, notifications, deliveries, tenants, rateLimits, prod := newTestReplayer(t)
	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusQueuedRateLimited}

	tenants.On("GetByID", mock.Anything, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", mock.Anything, tenantID, domain.ChannelEmail).Return(nil, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, n.ID, domain.StatusQueued).Return(assertErr)
	deliveries.On("SetStatus", mock.Anything, delivery.ID, domain.StatusQueued).Return(nil)

	r.replayOne(ctx, n)

	notifications.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestReplayOne_SetStatusError_LogsOnly(t *testing.T) {
	r, notifications, deliveries, tenants, rateLimits, prod := newTestReplayer(t)
	ctx := context.Background()
	tenantID := uuid.New()
	n := sampleNotification(tenantID)
	delivery := &domain.NotificationDelivery{ID: uuid.New(), NotificationID: n.ID, Channel: n.Channel, Status: domain.StatusQueuedRateLimited}

	tenants.On("GetByID", mock.Anything, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", mock.Anything, tenantID, domain.ChannelEmail).Return(nil, nil)
	deliveries.On("GetByNotificationID", mock.Anything, n.ID).Return([]*domain.NotificationDelivery{delivery}, nil)
	prod.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	notifications.On("UpdateStatus", mock.Anything, n.ID, domain.StatusQueued).Return(nil)
	deliveries.On("SetStatus", mock.Anything, delivery.ID, domain.StatusQueued).Return(assertErr)

	r.replayOne(ctx, n)

	notifications.AssertExpectations(t)
	deliveries.AssertExpectations(t)
}

func TestRun_TicksAndStopsOnContextCancel(t *testing.T) {
	r, notifications, _, _, _, _ := newTestReplayer(t)
	notifications.On("GetByStatus", mock.Anything, domain.StatusQueuedRateLimited, batchSize).Return([]*domain.Notification{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx, 5*time.Millisecond)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
