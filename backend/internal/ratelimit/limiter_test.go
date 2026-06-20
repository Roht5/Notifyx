package ratelimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/stretchr/testify/require"
)

func newTestLimiter(t *testing.T) (*Limiter, *mockTenantRepo, *mockRateLimitRepo, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	tenants := &mockTenantRepo{}
	rateLimits := &mockRateLimitRepo{}
	l := New(client, tenants, rateLimits, 5) // default 5/min
	return l, tenants, rateLimits, mr
}

func TestAllow_UnderLimit_UsesTenantConfiguredChannelLimit(t *testing.T) {
	l, tenants, rateLimits, _ := newTestLimiter(t)
	ctx := context.Background()
	tenantID := uuid.New()

	tenants.On("GetByID", ctx, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", ctx, tenantID, domain.ChannelEmail).
		Return(&domain.TenantRateLimit{TenantID: tenantID, Channel: domain.ChannelEmail, MaxPerMin: 2}, nil)

	allowed, err := l.Allow(ctx, tenantID, domain.ChannelEmail)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestAllow_FallsBackToDefaultWhenNoChannelConfig(t *testing.T) {
	l, tenants, rateLimits, _ := newTestLimiter(t)
	ctx := context.Background()
	tenantID := uuid.New()

	tenants.On("GetByID", ctx, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", ctx, tenantID, domain.ChannelSMS).Return(nil, nil)

	// defaultMax is 5 in newTestLimiter — first request should be allowed.
	allowed, err := l.Allow(ctx, tenantID, domain.ChannelSMS)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestAllow_ChannelLimitExhausted_Denies(t *testing.T) {
	l, tenants, rateLimits, _ := newTestLimiter(t)
	ctx := context.Background()
	tenantID := uuid.New()

	tenants.On("GetByID", ctx, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", ctx, tenantID, domain.ChannelPush).
		Return(&domain.TenantRateLimit{TenantID: tenantID, Channel: domain.ChannelPush, MaxPerMin: 1}, nil)

	allowed1, err := l.Allow(ctx, tenantID, domain.ChannelPush)
	require.NoError(t, err)
	require.True(t, allowed1)

	allowed2, err := l.Allow(ctx, tenantID, domain.ChannelPush)
	require.NoError(t, err)
	require.False(t, allowed2)
}

func TestAllow_GlobalCapExhausted_Denies(t *testing.T) {
	l, tenants, rateLimits, _ := newTestLimiter(t)
	ctx := context.Background()
	tenantID := uuid.New()

	tenants.On("GetByID", ctx, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 1}, nil)
	rateLimits.On("GetByChannel", ctx, tenantID, domain.ChannelEmail).
		Return(&domain.TenantRateLimit{TenantID: tenantID, Channel: domain.ChannelEmail, MaxPerMin: 100}, nil)
	rateLimits.On("GetByChannel", ctx, tenantID, domain.ChannelSMS).
		Return(&domain.TenantRateLimit{TenantID: tenantID, Channel: domain.ChannelSMS, MaxPerMin: 100}, nil)

	// First request on email channel consumes the global cap of 1.
	allowed1, err := l.Allow(ctx, tenantID, domain.ChannelEmail)
	require.NoError(t, err)
	require.True(t, allowed1)

	// A different channel for the same tenant should now be denied by the global cap.
	allowed2, err := l.Allow(ctx, tenantID, domain.ChannelSMS)
	require.NoError(t, err)
	require.False(t, allowed2)
}

func TestAllow_TenantLookupError_PropagatesError(t *testing.T) {
	l, tenants, _, _ := newTestLimiter(t)
	ctx := context.Background()
	tenantID := uuid.New()

	tenants.On("GetByID", ctx, tenantID).Return(nil, assertErr)

	allowed, err := l.Allow(ctx, tenantID, domain.ChannelEmail)
	require.Error(t, err)
	require.False(t, allowed)
}

func TestAllow_RateLimitLookupError_PropagatesError(t *testing.T) {
	l, tenants, rateLimits, _ := newTestLimiter(t)
	ctx := context.Background()
	tenantID := uuid.New()

	tenants.On("GetByID", ctx, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", ctx, tenantID, domain.ChannelEmail).Return(nil, assertErr)

	allowed, err := l.Allow(ctx, tenantID, domain.ChannelEmail)
	require.Error(t, err)
	require.False(t, allowed)
}

func TestAllow_RedisScriptError_PropagatesError(t *testing.T) {
	l, tenants, rateLimits, mr := newTestLimiter(t)
	ctx := context.Background()
	tenantID := uuid.New()

	tenants.On("GetByID", ctx, tenantID).Return(&domain.Tenant{ID: tenantID, GlobalRateCap: 100}, nil)
	rateLimits.On("GetByChannel", ctx, tenantID, domain.ChannelEmail).Return(nil, nil)

	mr.Close()

	allowed, err := l.Allow(ctx, tenantID, domain.ChannelEmail)
	require.Error(t, err)
	require.False(t, allowed)
}

var assertErr = errTest("boom")

type errTest string

func (e errTest) Error() string { return string(e) }
