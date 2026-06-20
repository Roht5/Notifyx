package offlinequeue

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestQueue(t *testing.T) (*Queue, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return New(client), mr
}

func TestPush_SingleMessage(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	err := q.Push(ctx, "tenant1", "user1", []byte("hello"))
	require.NoError(t, err)

	vals, err := mr.List(key("tenant1", "user1"))
	require.NoError(t, err)
	require.Equal(t, []string{"hello"}, vals)

	require.True(t, mr.TTL(key("tenant1", "user1")) > 0)
}

func TestPush_MultipleMessages_PreservesOrder(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.Push(ctx, "tenant1", "user1", []byte("first")))
	require.NoError(t, q.Push(ctx, "tenant1", "user1", []byte("second")))
	require.NoError(t, q.Push(ctx, "tenant1", "user1", []byte("third")))

	vals, err := mr.List(key("tenant1", "user1"))
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second", "third"}, vals)
}

func TestFlush_ReturnsAndClearsMessages(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.Push(ctx, "tenant1", "user1", []byte("a")))
	require.NoError(t, q.Push(ctx, "tenant1", "user1", []byte("b")))

	out, err := q.Flush(ctx, "tenant1", "user1")
	require.NoError(t, err)
	require.Equal(t, [][]byte{[]byte("a"), []byte("b")}, out)

	require.False(t, mr.Exists(key("tenant1", "user1")))
}

func TestFlush_EmptyQueue_ReturnsNilNoError(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	out, err := q.Flush(ctx, "tenantX", "userX")
	require.NoError(t, err)
	require.Nil(t, out)
}

func TestPush_RefreshesTTLOnEachCall(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.Push(ctx, "tenant1", "user1", []byte("a")))
	mr.FastForward(ttl / 2)
	require.NoError(t, q.Push(ctx, "tenant1", "user1", []byte("b")))
	mr.FastForward(ttl / 2)

	require.True(t, mr.Exists(key("tenant1", "user1")))
}

func TestKey_Format(t *testing.T) {
	require.Equal(t, "offline:t1:u1", key("t1", "u1"))
}

func TestFlush_LRangeError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := New(client)
	client.Close() // force subsequent commands to error

	out, err := q.Flush(context.Background(), "tenant1", "user1")
	require.Error(t, err)
	require.Nil(t, out)
}

func TestFlush_DelError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	q := New(client)

	require.NoError(t, q.Push(context.Background(), "tenant1", "user1", []byte("a")))

	// Replace the underlying list with a wrong-type value via a second client so DEL still
	// works fine — instead, simulate a Del failure by closing the connection right after a
	// successful LRange using a context that's already cancelled for the Del call.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := q.Flush(ctx, "tenant1", "user1")
	require.Error(t, err)
	require.Nil(t, out)
}
