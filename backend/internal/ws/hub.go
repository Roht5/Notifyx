// Package ws implements the in-app WebSocket delivery path: a Hub of live connections
// keyed by tenant+user, and the HTTP handler that upgrades and registers them.
package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = (pongWait * 9) / 10
	sendBufferSize = 32
)

// Conn wraps one client WebSocket connection. Writes go through a buffered channel so
// only writePump ever calls websocket.Conn methods that touch the write side — gorilla's
// Conn does not allow concurrent writers.
type Conn struct {
	tenantID string
	userID   string
	ws       *websocket.Conn
	send     chan []byte
	log      *logger.Logger
}

// Hub tracks one live connection per (tenant, user) in this process. There is no
// cross-instance fan-out — see decisions.md "in-app delivery is single-instance" — so
// IsOnline/SendToUser only ever reflect connections held by this process.
type Hub struct {
	mu    sync.RWMutex
	conns map[string]*Conn
	log   *logger.Logger
}

func NewHub(log *logger.Logger) *Hub {
	return &Hub{conns: make(map[string]*Conn), log: log}
}

func connKey(tenantID, userID string) string {
	return tenantID + ":" + userID
}

// register stores the connection, replacing (and closing) any existing connection for
// the same tenant+user — a reconnect supersedes the old socket rather than both being live.
func (h *Hub) register(tenantID, userID string, wsConn *websocket.Conn) *Conn {
	c := &Conn{
		tenantID: tenantID,
		userID:   userID,
		ws:       wsConn,
		send:     make(chan []byte, sendBufferSize),
		log:      h.log,
	}

	h.mu.Lock()
	if old, ok := h.conns[connKey(tenantID, userID)]; ok {
		close(old.send)
	}
	h.conns[connKey(tenantID, userID)] = c
	h.mu.Unlock()

	return c
}

// unregister removes c only if it is still the active connection for its key — guards
// against a stale unregister racing a newer reconnect's register.
func (h *Hub) unregister(c *Conn) {
	h.mu.Lock()
	if cur, ok := h.conns[connKey(c.tenantID, c.userID)]; ok && cur == c {
		delete(h.conns, connKey(c.tenantID, c.userID))
	}
	h.mu.Unlock()
}

// IsOnline reports whether tenant+user has a live connection on this process.
func (h *Hub) IsOnline(tenantID, userID string) bool {
	h.mu.RLock()
	_, ok := h.conns[connKey(tenantID, userID)]
	h.mu.RUnlock()
	return ok
}

// SendToUser enqueues payload for delivery to the user's active connection. Returns false
// if there is no connection, or its send buffer is full — callers should fall back to the
// offline queue in either case rather than blocking.
func (h *Hub) SendToUser(tenantID, userID string, payload []byte) bool {
	h.mu.RLock()
	c, ok := h.conns[connKey(tenantID, userID)]
	h.mu.RUnlock()
	if !ok {
		return false
	}

	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

// writePump serializes all writes to the underlying socket and sends periodic pings.
// Exits (and closes the socket) when send is closed or a write fails.
func (c *Conn) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	defer c.ws.Close()

	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump blocks reading frames until the client disconnects or the connection times
// out without a pong. Notifyx doesn't accept inbound client messages — this loop only
// exists to detect close and drive the pong-based liveness deadline.
func (c *Conn) readPump() {
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if _, _, err := c.ws.ReadMessage(); err != nil {
			return
		}
	}
}
