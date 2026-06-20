package presence

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestTracker(t *testing.T) (*Tracker, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return New(client), mr
}

func TestSetOnline(t *testing.T) {
	tracker, mr := newTestTracker(t)
	ctx := context.Background()

	err := tracker.SetOnline(ctx, "tenant1", "user1")
	require.NoError(t, err)

	val, err := mr.Get(key("tenant1", "user1"))
	require.NoError(t, err)
	require.Equal(t, "1", val)

	gotTTL := mr.TTL(key("tenant1", "user1"))
	require.Equal(t, ttl, gotTTL)
}

func TestSetOnline_RefreshesTTL(t *testing.T) {
	tracker, mr := newTestTracker(t)
	ctx := context.Background()

	require.NoError(t, tracker.SetOnline(ctx, "tenant1", "user1"))
	mr.FastForward(ttl / 2)
	require.NoError(t, tracker.SetOnline(ctx, "tenant1", "user1"))

	// key should still be present after the original TTL would have expired
	mr.FastForward(ttl / 2)
	require.True(t, mr.Exists(key("tenant1", "user1")))
}

func TestDelete(t *testing.T) {
	tracker, mr := newTestTracker(t)
	ctx := context.Background()

	require.NoError(t, tracker.SetOnline(ctx, "tenant1", "user1"))
	require.True(t, mr.Exists(key("tenant1", "user1")))

	err := tracker.Delete(ctx, "tenant1", "user1")
	require.NoError(t, err)
	require.False(t, mr.Exists(key("tenant1", "user1")))
}

func TestDelete_NonExistentKey(t *testing.T) {
	tracker, _ := newTestTracker(t)
	ctx := context.Background()

	// deleting a key that was never set should not error
	err := tracker.Delete(ctx, "tenantX", "userX")
	require.NoError(t, err)
}

func TestKey_Format(t *testing.T) {
	require.Equal(t, "presence:t1:u1", key("t1", "u1"))
}
