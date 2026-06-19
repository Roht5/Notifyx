package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
)

const windowSize = 60 * time.Second

// slidingWindowScript atomically checks and records a request against two sliding-window
// sorted sets (per-channel and global) in one round trip, so a request is only ever
// counted if both limits have room. Expired entries are trimmed first.
//
// KEYS[1] = per-channel rate key, KEYS[2] = global rate key
// ARGV[1] = now (unix nanoseconds), ARGV[2] = window size (nanoseconds)
// ARGV[3] = per-channel max, ARGV[4] = global max, ARGV[5] = unique member for this request
var slidingWindowScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local channelMax = tonumber(ARGV[3])
local globalMax = tonumber(ARGV[4])
local member = ARGV[5]

redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - window)
redis.call('ZREMRANGEBYSCORE', KEYS[2], 0, now - window)

local channelCount = redis.call('ZCARD', KEYS[1])
local globalCount = redis.call('ZCARD', KEYS[2])

if channelCount >= channelMax or globalCount >= globalMax then
    return 0
end

redis.call('ZADD', KEYS[1], now, member)
redis.call('ZADD', KEYS[2], now, member)
local windowSec = math.floor(window / 1000000000) + 1
redis.call('EXPIRE', KEYS[1], windowSec)
redis.call('EXPIRE', KEYS[2], windowSec)

return 1
`)

// Limiter enforces a per-tenant sliding-window rate limit, per channel and globally,
// backed by Redis sorted sets.
type Limiter struct {
	client     *redis.Client
	rateLimits *postgres.TenantRateLimitRepository
	defaultMax int
	defaultCap int
}

func New(client *redis.Client, rateLimits *postgres.TenantRateLimitRepository, defaultMaxPerMin, defaultGlobalCap int) *Limiter {
	return &Limiter{client: client, rateLimits: rateLimits, defaultMax: defaultMaxPerMin, defaultCap: defaultGlobalCap}
}

// Allow resolves tenantID's configured limits for channel (falling back to the server
// defaults if the tenant hasn't set any), then atomically checks and records the request
// against both the per-channel and global sliding windows. It returns false if either
// limit is currently exhausted.
func (l *Limiter) Allow(ctx context.Context, tenantID uuid.UUID, channel domain.Channel) (bool, error) {
	maxPerMin, globalCap, err := l.resolveLimits(ctx, tenantID, channel)
	if err != nil {
		return false, err
	}

	now := time.Now().UnixNano()
	channelKey := fmt.Sprintf("rate:%s:%s", tenantID, channel)
	globalKey := fmt.Sprintf("rate:%s:global", tenantID)
	member := fmt.Sprintf("%d-%s", now, uuid.NewString())

	result, err := slidingWindowScript.Run(ctx, l.client,
		[]string{channelKey, globalKey},
		now, windowSize.Nanoseconds(), maxPerMin, globalCap, member,
	).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit check: %w", err)
	}
	return result == 1, nil
}

func (l *Limiter) resolveLimits(ctx context.Context, tenantID uuid.UUID, channel domain.Channel) (maxPerMin, globalCap int, err error) {
	rl, err := l.rateLimits.GetByChannel(ctx, tenantID, channel)
	if err != nil {
		return 0, 0, fmt.Errorf("resolve rate limit config: %w", err)
	}
	if rl == nil {
		return l.defaultMax, l.defaultCap, nil
	}
	return rl.MaxPerMin, rl.GlobalCap, nil
}
