package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type mockEventBroadcaster struct {
	mu          sync.Mutex
	addCount    int
	removeCount int
	addCh       chan struct{}
	removeCh    chan struct{}
}

func (m *mockEventBroadcaster) AddClient(_ *websocket.Conn) {
	m.mu.Lock()
	m.addCount++
	m.mu.Unlock()
	select {
	case m.addCh <- struct{}{}:
	default:
	}
}

func (m *mockEventBroadcaster) RemoveClient(_ *websocket.Conn) {
	m.mu.Lock()
	m.removeCount++
	m.mu.Unlock()
	select {
	case m.removeCh <- struct{}{}:
	default:
	}
}

func TestWsHandler_HubUnavailable(t *testing.T) {
	h := NewWsHandler(nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/ws/events", nil)

	h.HandleEvents(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestWsHandler_HandleEvents_AddAndRemoveClient(t *testing.T) {
	mockHub := &mockEventBroadcaster{
		addCh:    make(chan struct{}, 1),
		removeCh: make(chan struct{}, 1),
	}

	server := httptest.NewServer(http.HandlerFunc(NewWsHandler(mockHub).HandleEvents))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	select {
	case <-mockHub.addCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting AddClient")
	}

	if err := conn.Close(websocket.StatusNormalClosure, "test done"); err != nil {
		t.Fatalf("close websocket: %v", err)
	}

	select {
	case <-mockHub.removeCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting RemoveClient")
	}
}
