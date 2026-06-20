package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/offlinequeue"
	"github.com/rohit-bagade/notifyx/internal/presence"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- test helpers ---

func testHandlerLogger(t *testing.T) *logger.Logger {
	t.Helper()
	l, err := logger.New("test", "error")
	require.NoError(t, err)
	return l
}

type mockAPIKeyRepo struct{ mock.Mock }

var _ postgres.APIKeyRepositoryInterface = (*mockAPIKeyRepo)(nil)

func (m *mockAPIKeyRepo) Create(ctx context.Context, tenantID uuid.UUID, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, tenantID, keyHash)
	k, _ := args.Get(0).(*domain.APIKey)
	return k, args.Error(1)
}

func (m *mockAPIKeyRepo) GetTenantByKeyHash(ctx context.Context, keyHash string) (*domain.Tenant, error) {
	args := m.Called(ctx, keyHash)
	t, _ := args.Get(0).(*domain.Tenant)
	return t, args.Error(1)
}

func (m *mockAPIKeyRepo) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, tenantID)
	k, _ := args.Get(0).([]*domain.APIKey)
	return k, args.Error(1)
}

func (m *mockAPIKeyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// testEnv wires up a Handler behind a real httptest server (so the real gorilla upgrade
// handshake runs end to end), along with miniredis-backed presence/queue and a mock
// API key repo for auth.
type testEnv struct {
	server   *httptest.Server
	repo     *mockAPIKeyRepo
	mr       *miniredis.Miniredis
	queue    *offlinequeue.Queue
	presence *presence.Tracker
}

func newTestEnv(t *testing.T, withRedis bool) *testEnv {
	t.Helper()

	repo := &mockAPIKeyRepo{}
	log := testHandlerLogger(t)
	hub := NewHub(log)

	var pres *presence.Tracker
	var q *offlinequeue.Queue
	var mr *miniredis.Miniredis

	if withRedis {
		mr = miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { client.Close() })
		pres = presence.New(client)
		q = offlinequeue.New(client)
	}

	h := NewHandler(hub, pres, q, repo, log)

	e := echo.New()
	e.GET("/ws/connect", h.Connect)
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)

	return &testEnv{server: srv, repo: repo, mr: mr, queue: q, presence: pres}
}

func (env *testEnv) wsURL(query url.Values) string {
	u := "ws" + env.server.URL[len("http"):] + "/ws/connect"
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// --- tests ---

func TestHandler_Connect_Success(t *testing.T) {
	env := newTestEnv(t, true)

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Name: "Acme"}
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("validkey")).Return(tenant, nil)

	q := url.Values{
		"tenantId": {tenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"validkey"},
	}

	cl, resp, err := websocket.DefaultDialer.Dial(env.wsURL(q), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// give the handler a moment to register and set presence before we assert on it
	require.Eventually(t, func() bool {
		return env.mr.Exists("presence:" + tenantID.String() + ":user1")
	}, time.Second, 10*time.Millisecond)

	env.repo.AssertExpectations(t)

	cl.Close()
	// Wait for the server-side handler's disconnect cleanup (presence delete) to finish
	// before miniredis's client gets closed by t.Cleanup.
	require.Eventually(t, func() bool {
		return !env.mr.Exists("presence:" + tenantID.String() + ":user1")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_Connect_MissingParams(t *testing.T) {
	env := newTestEnv(t, false)

	resp, err := http.Get(strings.Replace(env.wsURL(url.Values{"tenantId": {"t1"}}), "ws://", "http://", 1))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_Connect_InvalidAPIKey(t *testing.T) {
	env := newTestEnv(t, false)

	tenantID := uuid.New()
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("badkey")).Return(nil, postgres.ErrNotFound)

	q := url.Values{
		"tenantId": {tenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"badkey"},
	}

	resp, err := http.Get(strings.Replace(env.wsURL(q), "ws://", "http://", 1))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	env.repo.AssertExpectations(t)
}

func TestHandler_Connect_DBError(t *testing.T) {
	env := newTestEnv(t, false)

	tenantID := uuid.New()
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("key")).Return(nil, assertError("boom"))

	q := url.Values{
		"tenantId": {tenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"key"},
	}

	resp, err := http.Get(strings.Replace(env.wsURL(q), "ws://", "http://", 1))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandler_Connect_KeyBelongsToDifferentTenant(t *testing.T) {
	env := newTestEnv(t, false)

	actualTenantID := uuid.New()
	requestedTenantID := uuid.New()
	tenant := &domain.Tenant{ID: actualTenantID, Name: "Acme"}
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("validkey")).Return(tenant, nil)

	q := url.Values{
		"tenantId": {requestedTenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"validkey"},
	}

	resp, err := http.Get(strings.Replace(env.wsURL(q), "ws://", "http://", 1))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHandler_Connect_PresenceAndQueueNilSkipsBookkeeping(t *testing.T) {
	// Handler is constructed with nil presence/queue (REDIS_URL unset case); connect
	// should still succeed without panicking.
	env := newTestEnv(t, false)

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Name: "Acme"}
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("validkey")).Return(tenant, nil)

	q := url.Values{
		"tenantId": {tenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"validkey"},
	}

	cl, resp, err := websocket.DefaultDialer.Dial(env.wsURL(q), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	defer cl.Close()
}

func TestHandler_Connect_FlushesOfflineQueueOnConnect(t *testing.T) {
	env := newTestEnv(t, true)

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Name: "Acme"}
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("validkey")).Return(tenant, nil)

	ctx := context.Background()
	require.NoError(t, env.queue.Push(ctx, tenantID.String(), "user1", []byte("queued-message")))

	q := url.Values{
		"tenantId": {tenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"validkey"},
	}

	cl, resp, err := websocket.DefaultDialer.Dial(env.wsURL(q), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	cl.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, payload, err := cl.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "queued-message", string(payload))

	cl.Close()
	// Give the server-side handler goroutine a chance to finish its disconnect
	// cleanup (presence delete) before the miniredis client gets torn down by
	// t.Cleanup, avoiding a "client is closed" race in the log output.
	require.Eventually(t, func() bool {
		return !env.mr.Exists("presence:" + tenantID.String() + ":user1")
	}, time.Second, 10*time.Millisecond)
}

func TestHandler_Connect_RedisErrorsAreLoggedNotFatal(t *testing.T) {
	// When presence/queue calls fail (e.g. Redis became unreachable), Connect should log
	// the error but still proceed with the connection rather than failing the handshake.
	env := newTestEnv(t, true)

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Name: "Acme"}
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("validkey")).Return(tenant, nil)

	// Kill miniredis so SetOnline/Flush/Delete all fail with a connection error.
	env.mr.Close()

	q := url.Values{
		"tenantId": {tenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"validkey"},
	}

	cl, resp, err := websocket.DefaultDialer.Dial(env.wsURL(q), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	cl.Close()
	time.Sleep(50 * time.Millisecond) // let the server-side handler finish its disconnect path
}

func TestHandler_Connect_DisconnectClearsPresence(t *testing.T) {
	env := newTestEnv(t, true)

	tenantID := uuid.New()
	tenant := &domain.Tenant{ID: tenantID, Name: "Acme"}
	env.repo.On("GetTenantByKeyHash", mock.Anything, handlers.HashAPIKey("validkey")).Return(tenant, nil)

	q := url.Values{
		"tenantId": {tenantID.String()},
		"userId":   {"user1"},
		"apiKey":   {"validkey"},
	}

	cl, _, err := websocket.DefaultDialer.Dial(env.wsURL(q), nil)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return env.mr.Exists("presence:" + tenantID.String() + ":user1")
	}, time.Second, 10*time.Millisecond)

	cl.Close()

	require.Eventually(t, func() bool {
		return !env.mr.Exists("presence:" + tenantID.String() + ":user1")
	}, time.Second, 10*time.Millisecond)
}

// assertError is a minimal error type for DB-failure test cases.
type assertError string

func (e assertError) Error() string { return string(e) }
