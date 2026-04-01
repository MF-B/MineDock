package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"minedock/backend/internal/model"
)

func TestEventHub_AddRemoveClient(t *testing.T) {
	hub := newEventHub(nil, nil)
	serverConn, clientConn, cleanup := newWebSocketPair(t)
	defer cleanup()

	hub.AddClient(serverConn)
	if got := len(hub.snapshotClients()); got != 1 {
		t.Fatalf("expected 1 client, got %d", got)
	}

	hub.RemoveClient(serverConn)
	if got := len(hub.snapshotClients()); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}

	readCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, _, err := clientConn.Read(readCtx)
	if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatalf("expected normal closure, got %v", err)
	}
}

func TestEventHub_RefreshAndBroadcast_DeduplicateSnapshot(t *testing.T) {
	instances := []model.Instance{{ContainerID: "c1", Name: "alpha", Status: "Running"}}
	hub := newEventHub(nil, func(context.Context) ([]model.Instance, error) {
		return append([]model.Instance(nil), instances...), nil
	})

	serverConn, clientConn, cleanup := newWebSocketPair(t)
	defer cleanup()
	hub.AddClient(serverConn)

	if err := hub.refreshAndBroadcast(context.Background()); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	first := readWSMessage(t, clientConn)
	assertInstancesUpdatedPayload(t, first, "Running")

	nextMessageCh := readWSMessageAsync(clientConn)

	if err := hub.refreshAndBroadcast(context.Background()); err != nil {
		t.Fatalf("duplicate refresh: %v", err)
	}

	select {
	case result := <-nextMessageCh:
		if result.err != nil {
			t.Fatalf("unexpected websocket error: %v", result.err)
		}
		t.Fatalf("expected no message for duplicate snapshot, got %s", string(result.payload))
	case <-time.After(120 * time.Millisecond):
	}

	instances[0].Status = "Stopped"
	if err := hub.refreshAndBroadcast(context.Background()); err != nil {
		t.Fatalf("third refresh: %v", err)
	}

	select {
	case result := <-nextMessageCh:
		if result.err != nil {
			t.Fatalf("read websocket payload: %v", result.err)
		}
		assertInstancesUpdatedPayload(t, result.payload, "Stopped")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting message after snapshot change")
	}
}

func newWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()

	serverConnCh := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		serverConnCh <- conn
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))

	clientURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.Dial(context.Background(), clientURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}

	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case <-time.After(time.Second):
		_ = clientConn.Close(websocket.StatusAbnormalClosure, "timeout")
		server.Close()
		t.Fatal("timeout waiting for server websocket connection")
	}

	cleanup := func() {
		_ = clientConn.Close(websocket.StatusNormalClosure, "test cleanup")
		_ = serverConn.Close(websocket.StatusNormalClosure, "test cleanup")
		server.Close()
	}

	return serverConn, clientConn, cleanup
}

func readWSMessage(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read websocket payload: %v", err)
	}
	return payload
}

type wsReadResult struct {
	payload []byte
	err     error
}

func readWSMessageAsync(conn *websocket.Conn) <-chan wsReadResult {
	ch := make(chan wsReadResult, 1)
	go func() {
		_, payload, err := conn.Read(context.Background())
		ch <- wsReadResult{payload: payload, err: err}
	}()
	return ch
}

func assertInstancesUpdatedPayload(t *testing.T, payload []byte, wantStatus string) {
	t.Helper()

	var msg struct {
		Type string           `json:"type"`
		Data []model.Instance `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("decode websocket payload: %v", err)
	}
	if msg.Type != "instances_updated" {
		t.Fatalf("unexpected message type: %s", msg.Type)
	}
	if len(msg.Data) != 1 {
		t.Fatalf("unexpected data size: %d", len(msg.Data))
	}
	if msg.Data[0].Status != wantStatus {
		t.Fatalf("unexpected status: %s", msg.Data[0].Status)
	}
}
