// Package offlinequeue persists in-app messages for recipients who are offline at
// delivery time, so they can be flushed to the client on its next WebSocket connect.
package offlinequeue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// ttl bounds how long an undelivered queue survives if the recipient never reconnects —
// without it, a permanently-offline user's queue would grow in Redis forever.
const ttl = 7 * 24 * time.Hour

type Queue struct {
	client *redis.Client
}

func New(client *redis.Client) *Queue {
	return &Queue{client: client}
}

func key(tenantID, userID string) string {
	return "offline:" + tenantID + ":" + userID
}

// Push appends payload to tenant+user's offline queue and (re)sets its TTL.
func (q *Queue) Push(ctx context.Context, tenantID, userID string, payload []byte) error {
	k := key(tenantID, userID)
	pipe := q.client.Pipeline()
	pipe.RPush(ctx, k, payload)
	pipe.Expire(ctx, k, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Flush returns and clears every queued payload for tenant+user, oldest first.
// Returns a nil slice (not an error) if the queue is empty.
func (q *Queue) Flush(ctx context.Context, tenantID, userID string) ([][]byte, error) {
	k := key(tenantID, userID)

	vals, err := q.client.LRange(ctx, k, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		return nil, nil
	}
	if err := q.client.Del(ctx, k).Err(); err != nil {
		return nil, err
	}

	out := make([][]byte, len(vals))
	for i, v := range vals {
		out[i] = []byte(v)
	}
	return out, nil
}
