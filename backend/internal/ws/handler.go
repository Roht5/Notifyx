package ws

import (
	"errors"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/api/handlers"
	"github.com/rohit-bagade/notifyx/internal/offlinequeue"
	"github.com/rohit-bagade/notifyx/internal/presence"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// upgrader allows any origin, matching the CORSConfig.AllowOrigins = "*" already in
// routes.Setup — this API has no deployed-dashboard origin to restrict to yet.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler upgrades and registers WebSocket connections for in-app delivery.
// Presence and Queue may be nil when REDIS_URL isn't configured — in that case presence
// bookkeeping and offline-queue flush on connect are both skipped (every in-app message
// arriving while this client is briefly disconnected falls through to the Kafka
// retry/DLQ path instead, the same degrade-gracefully convention used elsewhere).
type Handler struct {
	hub        *Hub
	presence   *presence.Tracker
	queue      *offlinequeue.Queue
	apiKeyRepo postgres.APIKeyRepositoryInterface
	log        *logger.Logger
}

func NewHandler(hub *Hub, presenceTracker *presence.Tracker, queue *offlinequeue.Queue, apiKeyRepo postgres.APIKeyRepositoryInterface, log *logger.Logger) *Handler {
	return &Handler{hub: hub, presence: presenceTracker, queue: queue, apiKeyRepo: apiKeyRepo, log: log}
}

// Connect handles GET /ws/connect?tenantId=&userId=&apiKey=. The API key is passed as a
// query param rather than the usual X-API-Key header because browsers' WebSocket API
// cannot set custom headers on the handshake request. The key must belong to the tenant
// named in tenantId, preventing one tenant's key from opening a socket as another tenant.
func (h *Handler) Connect(c echo.Context) error {
	ctx := c.Request().Context()

	tenantID := c.QueryParam("tenantId")
	userID := c.QueryParam("userId")
	apiKey := c.QueryParam("apiKey")
	if tenantID == "" || userID == "" || apiKey == "" {
		return c.JSON(http.StatusBadRequest, handlers.ErrorResponse{Error: "tenantId, userId, and apiKey query params are required"})
	}

	tenant, err := h.apiKeyRepo.GetTenantByKeyHash(ctx, handlers.HashAPIKey(apiKey))
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return c.JSON(http.StatusUnauthorized, handlers.ErrorResponse{Error: "invalid API key"})
		}
		h.log.Errorw("ws auth db error", "error", err)
		return c.JSON(http.StatusInternalServerError, handlers.ErrorResponse{Error: "authentication failed"})
	}
	if tenant.ID.String() != tenantID {
		return c.JSON(http.StatusUnauthorized, handlers.ErrorResponse{Error: "apiKey does not belong to tenantId"})
	}

	wsConn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.log.Errorw("websocket upgrade failed", "error", err, "tenant_id", tenantID, "user_id", userID)
		return err
	}

	conn := h.hub.register(tenantID, userID, wsConn)
	if h.presence != nil {
		if err := h.presence.SetOnline(ctx, tenantID, userID); err != nil {
			h.log.Errorw("presence set online failed", "error", err, "tenant_id", tenantID, "user_id", userID)
		}
	}

	go conn.writePump()

	if h.queue != nil {
		pending, err := h.queue.Flush(ctx, tenantID, userID)
		if err != nil {
			h.log.Errorw("offline queue flush failed", "error", err, "tenant_id", tenantID, "user_id", userID)
		}
		for _, payload := range pending {
			h.hub.SendToUser(tenantID, userID, payload)
		}
	}

	h.log.Infow("websocket connected", "tenant_id", tenantID, "user_id", userID)

	conn.readPump() // blocks until the client disconnects or the connection times out

	h.hub.unregister(conn)
	if h.presence != nil {
		if err := h.presence.Delete(ctx, tenantID, userID); err != nil {
			h.log.Errorw("presence delete failed", "error", err, "tenant_id", tenantID, "user_id", userID)
		}
	}
	h.log.Infow("websocket disconnected", "tenant_id", tenantID, "user_id", userID)

	return nil
}
