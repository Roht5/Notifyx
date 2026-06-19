package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// reserveScript claims a key if absent or returns the existing value if present, in one
// round trip instead of a separate SETNX + GET — halves Redis latency on duplicate hits,
// which matters most when talking to a serverless/WAN-hop Redis like Upstash.
//
// KEYS[1] = dedup key, ARGV[1] = notification ID, ARGV[2] = ttl in seconds
// Returns {1, nil} if claimed, {0, existing value} if a reservation already exists.
var reserveScript = redis.NewScript(`
if redis.call('SETNX', KEYS[1], ARGV[1]) == 1 then
    redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
    return {1, ''}
else
    return {0, redis.call('GET', KEYS[1])}
end
`)

// Deduplicator implements idempotency-key based deduplication: the first request to use
// a given key wins and its notification ID is cached for ttl; any repeat within that
// window is told the original notification's ID instead of creating a new one.
type Deduplicator struct {
	client *redis.Client
	ttl    time.Duration
}

func New(client *redis.Client, ttl time.Duration) *Deduplicator {
	return &Deduplicator{client: client, ttl: ttl}
}

// Reserve atomically claims idempotencyKey for notificationID.
//
// claimed=true means this is the first request with this key — the caller must create
// the notification using exactly notificationID, since that's the ID now cached against
// the key. claimed=false means a request with this key already went through — existingID
// is that original notification's ID, which the caller should return instead of creating
// a new one.
func (d *Deduplicator) Reserve(ctx context.Context, idempotencyKey string, notificationID uuid.UUID) (claimed bool, existingID uuid.UUID, err error) {
	key := redisKey(idempotencyKey)
	res, err := reserveScript.Run(ctx, d.client, []string{key}, notificationID.String(), int(d.ttl.Seconds())).Result()
	if err != nil {
		return false, uuid.Nil, fmt.Errorf("dedup reserve: %w", err)
	}

	pair, ok := res.([]interface{})
	if !ok || len(pair) != 2 {
		return false, uuid.Nil, fmt.Errorf("dedup reserve: unexpected script result %T", res)
	}
	claimedFlag, _ := pair[0].(int64)
	if claimedFlag == 1 {
		return true, uuid.Nil, nil
	}

	val, _ := pair[1].(string)
	existing, err := uuid.Parse(val)
	if err != nil {
		return false, uuid.Nil, fmt.Errorf("parse existing notification id: %w", err)
	}
	return false, existing, nil
}

// Release removes a reservation. Call this if creating the notification fails after
// Reserve succeeded, so the idempotency key doesn't sit unusable for the TTL window
// pointing at a notification that was never persisted.
func (d *Deduplicator) Release(ctx context.Context, idempotencyKey string) error {
	if err := d.client.Del(ctx, redisKey(idempotencyKey)).Err(); err != nil {
		return fmt.Errorf("dedup release: %w", err)
	}
	return nil
}

// redisKey hashes the client-supplied idempotency key with SHA-256 before using it in the
// Redis key, so an attacker (or buggy client) can't bloat Redis memory by passing an
// arbitrarily long string as the idempotency key — every key is a fixed 64 hex chars.
func redisKey(idempotencyKey string) string {
	sum := sha256.Sum256([]byte(idempotencyKey))
	return "dedup:" + hex.EncodeToString(sum[:])
}
