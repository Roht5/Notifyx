// Package presence records which (tenant, user) pairs currently have an open WebSocket
// connection. It is a Redis-backed record for observability/future multi-instance use —
// the actual online/offline delivery decision is made by ws.Hub.IsOnline, which is
// authoritative for this single-instance deployment. See decisions.md.
package presence

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// ttl is generous relative to the Hub's ping interval so a key never expires while a
// connection is genuinely still alive and pinging.
const ttl = 2 * time.Minute

type Tracker struct {
	client *redis.Client
}

func New(client *redis.Client) *Tracker {
	return &Tracker{client: client}
}

func key(tenantID, userID string) string {
	return "presence:" + tenantID + ":" + userID
}

// SetOnline records tenant+user as connected, refreshing the TTL.
func (t *Tracker) SetOnline(ctx context.Context, tenantID, userID string) error {
	return t.client.Set(ctx, key(tenantID, userID), "1", ttl).Err()
}

// Delete removes the presence record on disconnect.
func (t *Tracker) Delete(ctx context.Context, tenantID, userID string) error {
	return t.client.Del(ctx, key(tenantID, userID)).Err()
}
