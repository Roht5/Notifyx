package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const ttl = 24 * time.Hour

// Deduplicator implements idempotency-key based deduplication: the first request to use
// a given key wins and its notification ID is cached for ttl; any repeat within that
// window is told the original notification's ID instead of creating a new one.
type Deduplicator struct {
	client *redis.Client
}

func New(client *redis.Client) *Deduplicator {
	return &Deduplicator{client: client}
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
	ok, err := d.client.SetNX(ctx, key, notificationID.String(), ttl).Result()
	if err != nil {
		return false, uuid.Nil, fmt.Errorf("dedup reserve: %w", err)
	}
	if ok {
		return true, uuid.Nil, nil
	}

	val, err := d.client.Get(ctx, key).Result()
	if err != nil {
		return false, uuid.Nil, fmt.Errorf("dedup get existing: %w", err)
	}
	existing, err := uuid.Parse(val)
	if err != nil {
		return false, uuid.Nil, fmt.Errorf("parse existing notification id: %w", err)
	}
	return false, existing, nil
}

// Release removes a reservation. Call this if creating the notification fails after
// Reserve succeeded, so the idempotency key doesn't sit unusable for 24h pointing at a
// notification that was never persisted.
func (d *Deduplicator) Release(ctx context.Context, idempotencyKey string) error {
	if err := d.client.Del(ctx, redisKey(idempotencyKey)).Err(); err != nil {
		return fmt.Errorf("dedup release: %w", err)
	}
	return nil
}

func redisKey(idempotencyKey string) string {
	return "dedup:" + idempotencyKey
}
