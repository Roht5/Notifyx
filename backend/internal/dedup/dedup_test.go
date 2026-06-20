package dedup

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestDeduplicator(t *testing.T, ttl time.Duration) (*Deduplicator, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return New(client, ttl), mr
}

func TestReserve_FirstCall_Claims(t *testing.T) {
	d, _ := newTestDeduplicator(t, time.Minute)
	ctx := context.Background()
	id := uuid.New()

	claimed, existing, err := d.Reserve(ctx, "idem-key-1", id)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, uuid.Nil, existing)
}

func TestReserve_SecondCall_ReturnsExistingID(t *testing.T) {
	d, _ := newTestDeduplicator(t, time.Minute)
	ctx := context.Background()
	firstID := uuid.New()
	secondID := uuid.New()

	claimed, _, err := d.Reserve(ctx, "idem-key-1", firstID)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed2, existing, err := d.Reserve(ctx, "idem-key-1", secondID)
	require.NoError(t, err)
	require.False(t, claimed2)
	require.Equal(t, firstID, existing)
}

func TestReserve_DifferentKeys_BothClaim(t *testing.T) {
	d, _ := newTestDeduplicator(t, time.Minute)
	ctx := context.Background()

	claimed1, _, err := d.Reserve(ctx, "key-a", uuid.New())
	require.NoError(t, err)
	require.True(t, claimed1)

	claimed2, _, err := d.Reserve(ctx, "key-b", uuid.New())
	require.NoError(t, err)
	require.True(t, claimed2)
}

func TestReserve_TTLExpiry_AllowsReclaim(t *testing.T) {
	d, mr := newTestDeduplicator(t, 5*time.Second)
	ctx := context.Background()
	firstID := uuid.New()
	secondID := uuid.New()

	claimed, _, err := d.Reserve(ctx, "idem-key-ttl", firstID)
	require.NoError(t, err)
	require.True(t, claimed)

	mr.FastForward(6 * time.Second)

	claimed2, existing, err := d.Reserve(ctx, "idem-key-ttl", secondID)
	require.NoError(t, err)
	require.True(t, claimed2)
	require.Equal(t, uuid.Nil, existing)
}

func TestRelease_RemovesReservation(t *testing.T) {
	d, _ := newTestDeduplicator(t, time.Minute)
	ctx := context.Background()
	firstID := uuid.New()
	secondID := uuid.New()

	claimed, _, err := d.Reserve(ctx, "idem-key-release", firstID)
	require.NoError(t, err)
	require.True(t, claimed)

	err = d.Release(ctx, "idem-key-release")
	require.NoError(t, err)

	claimed2, existing, err := d.Reserve(ctx, "idem-key-release", secondID)
	require.NoError(t, err)
	require.True(t, claimed2)
	require.Equal(t, uuid.Nil, existing)
}

func TestRelease_NonExistentKey_NoError(t *testing.T) {
	d, _ := newTestDeduplicator(t, time.Minute)
	ctx := context.Background()

	err := d.Release(ctx, "never-reserved")
	require.NoError(t, err)
}

func TestRedisKey_IsHashedAndStable(t *testing.T) {
	k1 := redisKey("some-idempotency-key")
	k2 := redisKey("some-idempotency-key")
	k3 := redisKey("different-key")

	require.Equal(t, k1, k2)
	require.NotEqual(t, k1, k3)
	require.Contains(t, k1, "dedup:")
	require.Len(t, k1, len("dedup:")+64)
}

func TestReserve_ScriptError_ConnectionClosed(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	client.Close()
	d := New(client, time.Minute)

	claimed, existing, err := d.Reserve(context.Background(), "key", uuid.New())
	require.Error(t, err)
	require.False(t, claimed)
	require.Equal(t, uuid.Nil, existing)
}

func TestRelease_ConnectionClosed_ReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	client.Close()
	d := New(client, time.Minute)

	err := d.Release(context.Background(), "key")
	require.Error(t, err)
}

func TestReserve_ExistingValueIsNotAUUID_ReturnsError(t *testing.T) {
	d, mr := newTestDeduplicator(t, time.Minute)
	ctx := context.Background()

	// Manually seed the dedup key with a non-UUID value to force the existing-value
	// uuid.Parse to fail inside Reserve's "already reserved" branch.
	rk := redisKey("bad-value-key")
	require.NoError(t, mr.Set(rk, "not-a-uuid"))

	claimed, existing, err := d.Reserve(ctx, "bad-value-key", uuid.New())
	require.Error(t, err)
	require.False(t, claimed)
	require.Equal(t, uuid.Nil, existing)
}
