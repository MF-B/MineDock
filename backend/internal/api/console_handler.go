package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/docker/docker/pkg/stdcopy"

	"minedock/backend/internal/service"
)

const consoleWriteTimeout = 3 * time.Second

// ContainerConsole 定义控制台处理器依赖的会话打开能力。
type ContainerConsole interface {
	Open(ctx context.Context, containerID string) (*service.ConsoleSession, error)
}

// ConsoleHandler 暴露容器控制台 WebSocket 处理器。
type ConsoleHandler struct {
	console ContainerConsole
}

// NewConsoleHandler 创建 ConsoleHandler。
func NewConsoleHandler(console ContainerConsole) *ConsoleHandler {
	return &ConsoleHandler{console: console}
}

// HandleConsole 处理 GET /api/ws/console/{id}。
func (h *ConsoleHandler) HandleConsole(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.console == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "console service unavailable"})
		return
	}

	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		return
	}

	session, err := h.console.Open(r.Context(), id)
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer session.Close()

	errCh := make(chan error, 2)

	go func() {
		errCh <- pipeConsoleToWebSocket(ctx, conn, session)
	}()

	if session.Input != nil {
		go func() {
			errCh <- pipeWebSocketToConsole(ctx, conn, session.Input)
		}()
	}

	if err := <-errCh; err != nil {
		if code := websocket.CloseStatus(err); code == websocket.StatusNormalClosure || code == websocket.StatusGoingAway {
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return
		}
		_ = conn.Close(websocket.StatusInternalError, "console bridge failed")
		return
	}

	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func pipeConsoleToWebSocket(
	ctx context.Context,
	conn *websocket.Conn,
	session *service.ConsoleSession,
) error {
	writer := &wsBinaryWriter{ctx: ctx, conn: conn}
	var err error
	if session.TTY {
		_, err = io.Copy(writer, session.Output)
	} else {
		_, err = stdcopy.StdCopy(writer, writer, session.Output)
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func pipeWebSocketToConsole(ctx context.Context, conn *websocket.Conn, input io.Writer) error {
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}

		if typ != websocket.MessageText && typ != websocket.MessageBinary {
			continue
		}

		if len(data) == 0 {
			continue
		}

		if _, err := input.Write(data); err != nil {
			return fmt.Errorf("write stdin: %w", err)
		}
	}
}

type wsBinaryWriter struct {
	ctx  context.Context
	conn *websocket.Conn
}

func (w *wsBinaryWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	buf := make([]byte, len(p))
	copy(buf, p)

	writeCtx, cancel := context.WithTimeout(w.ctx, consoleWriteTimeout)
	defer cancel()

	if err := w.conn.Write(writeCtx, websocket.MessageBinary, buf); err != nil {
		return 0, err
	}

	return len(p), nil
}
