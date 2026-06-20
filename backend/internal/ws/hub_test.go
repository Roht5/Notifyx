package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rohit-bagade/notifyx/pkg/logger"
	"github.com/stretchr/testify/require"
)

func testHubLogger(t *testing.T) *logger.Logger {
	l, err := logger.New("test", "error")
	require.NoError(t, err)
	return l
}

// testWSPair spins up a real httptest server that immediately upgrades any incoming
// request to a WebSocket, and returns the server-side *websocket.Conn (suitable for
// Hub.register) along with the dialed client-side conn used to drive the other end and
// a cleanup func.
func testWSPair(t *testing.T) (server *websocket.Conn, client *websocket.Conn) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		serverConnCh <- c
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + srv.URL[len("http"):]
	cl, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { cl.Close() })

	sc := <-serverConnCh
	t.Cleanup(func() { sc.Close() })

	return sc, cl
}

func TestHub_RegisterAndIsOnline(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc, _ := testWSPair(t)

	require.False(t, h.IsOnline("t1", "u1"))

	conn := h.register("t1", "u1", sc)
	require.NotNil(t, conn)
	require.True(t, h.IsOnline("t1", "u1"))
	require.False(t, h.IsOnline("t1", "u2"))
	require.False(t, h.IsOnline("t2", "u1"))
}

func TestHub_Unregister(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc, _ := testWSPair(t)

	conn := h.register("t1", "u1", sc)
	require.True(t, h.IsOnline("t1", "u1"))

	h.unregister(conn)
	require.False(t, h.IsOnline("t1", "u1"))
}

func TestHub_Unregister_StaleNoOp(t *testing.T) {
	// A stale unregister (for a connection that has already been superseded by a
	// reconnect) must not remove the newer, still-active connection.
	h := NewHub(testHubLogger(t))
	sc1, _ := testWSPair(t)
	sc2, _ := testWSPair(t)

	oldConn := h.register("t1", "u1", sc1)
	newConn := h.register("t1", "u1", sc2)
	require.NotEqual(t, oldConn, newConn)
	require.True(t, h.IsOnline("t1", "u1"))

	// Unregistering the stale (replaced) connection should be a no-op.
	h.unregister(oldConn)
	require.True(t, h.IsOnline("t1", "u1"))

	// Unregistering the current connection removes it.
	h.unregister(newConn)
	require.False(t, h.IsOnline("t1", "u1"))
}

func TestHub_Register_ReplacesAndClosesOldSendChannel(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc1, _ := testWSPair(t)
	sc2, _ := testWSPair(t)

	oldConn := h.register("t1", "u1", sc1)
	h.register("t1", "u1", sc2)

	// old connection's send channel should now be closed.
	_, ok := <-oldConn.send
	require.False(t, ok)
}

func TestHub_SendToUser_NoConnection(t *testing.T) {
	h := NewHub(testHubLogger(t))
	ok := h.SendToUser("t1", "u1", []byte("hello"))
	require.False(t, ok)
}

func TestHub_SendToUser_Success(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc, _ := testWSPair(t)
	conn := h.register("t1", "u1", sc)

	ok := h.SendToUser("t1", "u1", []byte("hello"))
	require.True(t, ok)

	select {
	case payload := <-conn.send:
		require.Equal(t, []byte("hello"), payload)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for payload on send channel")
	}
}

func TestHub_SendToUser_BufferFull(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc, _ := testWSPair(t)
	h.register("t1", "u1", sc)

	// Fill the send buffer without draining it (no writePump running).
	for i := 0; i < sendBufferSize; i++ {
		require.True(t, h.SendToUser("t1", "u1", []byte("msg")))
	}

	// Buffer is now full; the next send should report failure rather than blocking.
	ok := h.SendToUser("t1", "u1", []byte("overflow"))
	require.False(t, ok)
}

func TestConn_WritePump_DeliversMessage(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc, cl := testWSPair(t)
	conn := h.register("t1", "u1", sc)

	go conn.writePump()

	require.True(t, h.SendToUser("t1", "u1", []byte("hello world")))

	cl.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, payload, err := cl.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, msgType)
	require.Equal(t, "hello world", string(payload))
}

func TestConn_WritePump_ClosesOnSendChannelClose(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc, cl := testWSPair(t)
	conn := h.register("t1", "u1", sc)

	done := make(chan struct{})
	go func() {
		conn.writePump()
		close(done)
	}()

	// Registering a new connection for the same key closes the old conn's send channel,
	// which should cause writePump to send a close frame and return.
	sc2, _ := testWSPair(t)
	h.register("t1", "u1", sc2)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("writePump did not exit after send channel closed")
	}

	cl.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := cl.ReadMessage()
	require.Error(t, err) // expect close error
}

func TestConn_ReadPump_ExitsOnClientDisconnect(t *testing.T) {
	h := NewHub(testHubLogger(t))
	sc, cl := testWSPair(t)
	conn := h.register("t1", "u1", sc)

	done := make(chan struct{})
	go func() {
		conn.readPump()
		close(done)
	}()

	cl.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("readPump did not exit after client disconnect")
	}
}

func TestConnKey(t *testing.T) {
	require.Equal(t, "t1:u1", connKey("t1", "u1"))
}
