package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"minedock/backend/internal/model"
)

const (
	eventHubDebounceWindow = 150 * time.Millisecond
	eventHubWriteTimeout   = 3 * time.Second
	eventHubMaxBackoff     = 30 * time.Second
)

// dockerEventClient 定义 EventHub 依赖的 Docker Events 能力。
type dockerEventClient interface {
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
}

type instancesUpdatedMessage struct {
	Type string           `json:"type"`
	Data []model.Instance `json:"data"`
}

// EventHub 管理 WebSocket 连接并将 Docker 事件广播给所有客户端。
type EventHub struct {
	cli    dockerEventClient
	listFn func(ctx context.Context) ([]model.Instance, error)

	mu           sync.RWMutex
	clients      map[*websocket.Conn]struct{}
	lastSnapshot []byte
	lastPayload  []byte

	debounceWindow time.Duration
	writeTimeout   time.Duration
}

// NewEventHub 创建 EventHub。
// listFn 是获取完整实例列表的回调（注入 DockerService.ListInstances）。
func NewEventHub(cli *client.Client, listFn func(ctx context.Context) ([]model.Instance, error)) *EventHub {
	return newEventHub(cli, listFn)
}

func newEventHub(cli dockerEventClient, listFn func(ctx context.Context) ([]model.Instance, error)) *EventHub {
	return &EventHub{
		cli:            cli,
		listFn:         listFn,
		clients:        make(map[*websocket.Conn]struct{}),
		debounceWindow: eventHubDebounceWindow,
		writeTimeout:   eventHubWriteTimeout,
	}
}

// Run 启动 Docker Events 监听循环，ctx 取消时退出。
func (h *EventHub) Run(ctx context.Context) {
	if h == nil {
		return
	}
	defer h.closeAllClients(websocket.StatusGoingAway, "server shutdown")

	backoff := time.Second
	for {
		healthy, err := h.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if healthy {
			backoff = time.Second
		}
		if err != nil {
			slog.Warn("event hub run once failed", "error", err)
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if backoff < eventHubMaxBackoff {
			backoff *= 2
			if backoff > eventHubMaxBackoff {
				backoff = eventHubMaxBackoff
			}
		}
	}
}

func (h *EventHub) runOnce(ctx context.Context) (bool, error) {
	if h.cli == nil {
		return false, fmt.Errorf("docker client is nil")
	}
	if h.listFn == nil {
		return false, fmt.Errorf("list function is nil")
	}

	healthy := false

	args := filters.NewArgs()
	args.Add("type", "container")
	args.Add("label", managedLabelKey+"="+managedLabelValue)
	for _, action := range []string{"start", "stop", "die", "destroy", "kill", "restart"} {
		args.Add("event", action)
	}

	msgCh, errCh := h.cli.Events(ctx, events.ListOptions{Filters: args})

	// 每次连接（含重连）成功后立即推送一次全量快照。
	if err := h.refreshAndBroadcast(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("event hub initial snapshot failed", "error", err)
	} else if err == nil {
		healthy = true
	}

	timer := time.NewTimer(time.Hour)
	stopAndDrainTimer(timer)
	timerCh := (<-chan time.Time)(nil)
	pending := false

	for {
		select {
		case <-ctx.Done():
			if timerCh != nil {
				stopAndDrainTimer(timer)
			}
			return healthy, nil
		case _, ok := <-msgCh:
			if !ok {
				return healthy, fmt.Errorf("docker events channel closed")
			}
			pending = true
			healthy = true
			timerCh = resetTimer(timer, h.debounceWindow)
		case err, ok := <-errCh:
			if !ok {
				return healthy, fmt.Errorf("docker events error channel closed")
			}
			if err != nil {
				return healthy, fmt.Errorf("docker events stream error: %w", err)
			}
		case <-timerCh:
			timerCh = nil
			if !pending {
				continue
			}
			pending = false
			if err := h.refreshAndBroadcast(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("event hub broadcast snapshot failed", "error", err)
			} else if err == nil {
				healthy = true
			}
		}
	}
}

func (h *EventHub) refreshAndBroadcast(ctx context.Context) error {
	instances, err := h.listFn(ctx)
	if err != nil {
		return fmt.Errorf("list instances: %w", err)
	}

	snapshot, payload, err := buildEventPayload(instances)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if h.shouldSkipSnapshot(snapshot) {
		return nil
	}

	h.updateSnapshot(snapshot, payload)
	h.broadcastPayload(payload)
	return nil
}

func buildEventPayload(instances []model.Instance) ([]byte, []byte, error) {
	ordered := append([]model.Instance(nil), instances...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ContainerID != ordered[j].ContainerID {
			return ordered[i].ContainerID < ordered[j].ContainerID
		}
		return ordered[i].Name < ordered[j].Name
	})

	snapshot, err := json.Marshal(ordered)
	if err != nil {
		return nil, nil, err
	}

	payload, err := json.Marshal(instancesUpdatedMessage{Type: "instances_updated", Data: ordered})
	if err != nil {
		return nil, nil, err
	}

	return snapshot, payload, nil
}

func (h *EventHub) shouldSkipSnapshot(snapshot []byte) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return bytes.Equal(h.lastSnapshot, snapshot)
}

func (h *EventHub) updateSnapshot(snapshot []byte, payload []byte) {
	h.mu.Lock()
	h.lastSnapshot = append([]byte(nil), snapshot...)
	h.lastPayload = append([]byte(nil), payload...)
	h.mu.Unlock()
}

// AddClient 注册一个 WebSocket 客户端连接。
func (h *EventHub) AddClient(conn *websocket.Conn) {
	if h == nil || conn == nil {
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	payload := append([]byte(nil), h.lastPayload...)
	h.mu.Unlock()

	if len(payload) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.writeTimeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		h.removeClient(conn, websocket.StatusInternalError, "send initial snapshot failed")
	}
}

// RemoveClient 移除一个 WebSocket 客户端连接。
func (h *EventHub) RemoveClient(conn *websocket.Conn) {
	if h == nil || conn == nil {
		return
	}
	h.removeClient(conn, websocket.StatusNormalClosure, "client disconnected")
}

func (h *EventHub) removeClient(conn *websocket.Conn, code websocket.StatusCode, reason string) {
	h.mu.Lock()
	_, exists := h.clients[conn]
	if exists {
		delete(h.clients, conn)
	}
	h.mu.Unlock()

	if exists {
		_ = conn.Close(code, reason)
	}
}

func (h *EventHub) broadcastPayload(payload []byte) {
	clients := h.snapshotClients()
	for _, conn := range clients {
		ctx, cancel := context.WithTimeout(context.Background(), h.writeTimeout)
		err := conn.Write(ctx, websocket.MessageText, payload)
		cancel()
		if err != nil {
			h.removeClient(conn, websocket.StatusInternalError, "write failed")
		}
	}
}

func (h *EventHub) snapshotClients() []*websocket.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
	}
	return clients
}

func (h *EventHub) closeAllClients(code websocket.StatusCode, reason string) {
	h.mu.Lock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
	}
	h.clients = make(map[*websocket.Conn]struct{})
	h.mu.Unlock()

	for _, conn := range clients {
		_ = conn.Close(code, reason)
	}
}

func resetTimer(timer *time.Timer, d time.Duration) <-chan time.Time {
	stopAndDrainTimer(timer)
	timer.Reset(d)
	return timer.C
}

func stopAndDrainTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
