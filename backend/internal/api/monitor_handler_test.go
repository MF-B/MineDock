package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"minedock/backend/internal/model"
)

type mockServerMonitor struct {
	metrics *model.ServerMetrics
	err     error
}

func (m *mockServerMonitor) GetServerMetrics(_ context.Context) (*model.ServerMetrics, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.metrics, nil
}

func newMonitorTestRouter(monitor ServerMonitor) http.Handler {
	h := NewHandler(&mockService{
		listFn: func(_ context.Context) ([]model.Instance, error) {
			return []model.Instance{}, nil
		},
		createFn: func(_ context.Context, _, _ string, _ map[string]string, _ []model.PortMapping, _ *model.ResourceLimits) (string, error) {
			return "", nil
		},
		startFn:  func(_ context.Context, _ string) error { return nil },
		stopFn:   func(_ context.Context, _ string) error { return nil },
		deleteFn: func(_ context.Context, _ string, _ bool) error { return nil },
	})
	return NewRouter(h, nil, nil, nil, nil, nil, NewMonitorHandler(monitor))
}

func TestMonitorServer_Success(t *testing.T) {
	router := newMonitorTestRouter(&mockServerMonitor{
		metrics: &model.ServerMetrics{
			Timestamp: 1234,
			CPU: model.ServerCPUMetrics{
				Percent:      12.5,
				Cores:        []float64{10, 15},
				LogicalCores: 2,
				Model:        "Test CPU",
			},
			Memory: model.ServerMemoryMetrics{
				Percent:    50,
				UsedBytes:  1024,
				TotalBytes: 2048,
				Model:      "System Memory 2 GB",
			},
			Disks: []model.ServerDiskMetrics{{
				ID:         "/",
				Label:      "Disk 1",
				Name:       "/ (/dev/sda1)",
				Mountpoint: "/",
				Percent:    25,
				TotalBytes: 4096,
				UsedBytes:  1024,
			}},
			Network: model.ServerNetworkMetrics{Name: "eth0", RxBps: 100, TxBps: 200},
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/monitor/server", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got model.ServerMetrics
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Timestamp != 1234 || got.CPU.Model != "Test CPU" || got.Network.Name != "eth0" {
		t.Fatalf("unexpected response: %+v", got)
	}
	if len(got.Disks) != 1 || got.Disks[0].ID != "/" {
		t.Fatalf("unexpected disks: %+v", got.Disks)
	}
}

func TestMonitorServer_Error(t *testing.T) {
	router := newMonitorTestRouter(&mockServerMonitor{err: errors.New("collect failed")})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/monitor/server", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
