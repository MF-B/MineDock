package api

import (
	"net/http"

	"github.com/coder/websocket"
)

// EventBroadcaster 定义 WebSocket Handler 依赖的事件广播操作。
type EventBroadcaster interface {
	AddClient(conn *websocket.Conn)
	RemoveClient(conn *websocket.Conn)
}

// WsHandler 暴露 WebSocket 相关 HTTP 处理器。
type WsHandler struct {
	hub EventBroadcaster
}

// NewWsHandler 创建 WsHandler。
func NewWsHandler(hub EventBroadcaster) *WsHandler {
	return &WsHandler{hub: hub}
}

// HandleEvents 处理 GET /api/ws/events，将 HTTP 连接升级为 WebSocket。
func (h *WsHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.hub == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "event hub unavailable"})
		return
	}

	// 仅支持同源 WebSocket，保持默认 Origin 校验行为。
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		return
	}

	h.hub.AddClient(conn)
	defer h.hub.RemoveClient(conn)

	for {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
	}
}
