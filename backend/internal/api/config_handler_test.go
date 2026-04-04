package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"minedock/backend/internal/model"
	"minedock/backend/internal/service"
)

type mockConfigurator struct {
	getFn    func(ctx context.Context, containerID string) (*service.InstanceConfig, error)
	updateFn func(ctx context.Context, containerID string, params map[string]string, ports []model.PortMapping) (string, error)
}

func (m *mockConfigurator) GetInstanceConfig(ctx context.Context, containerID string) (*service.InstanceConfig, error) {
	if m == nil || m.getFn == nil {
		return nil, errTest
	}
	return m.getFn(ctx, containerID)
}

func (m *mockConfigurator) UpdateInstanceConfig(
	ctx context.Context,
	containerID string,
	params map[string]string,
	ports []model.PortMapping,
) (string, error) {
	if m == nil || m.updateFn == nil {
		return "", errTest
	}
	return m.updateFn(ctx, containerID, params, ports)
}

func newConfigTestRouter(cfg *mockConfigurator) http.Handler {
	h := NewHandler(&mockService{
		listFn: func(_ context.Context) ([]model.Instance, error) {
			return []model.Instance{}, nil
		},
		createFn: func(_ context.Context, _, _ string, _ map[string]string, _ []model.PortMapping) (string, error) {
			return "", nil
		},
		startFn: func(_ context.Context, _ string) error { return nil },
		stopFn:  func(_ context.Context, _ string) error { return nil },
		deleteFn: func(_ context.Context, _ string) error {
			return nil
		},
	})

	return NewRouter(h, nil, nil, nil, NewConfigHandler(cfg))
}

func TestGetConfig_Success(t *testing.T) {
	router := newConfigTestRouter(&mockConfigurator{
		getFn: func(_ context.Context, containerID string) (*service.InstanceConfig, error) {
			if containerID != "c1" {
				t.Fatalf("unexpected container id: %s", containerID)
			}
			return &service.InstanceConfig{
				GameID: "minecraft-java",
				Status: "Stopped",
				Ports:  []model.PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
				Params: map[string]string{"MAX_PLAYERS": "20"},
			}, nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/instances/c1/config", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got service.InstanceConfig
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.GameID != "minecraft-java" || got.Params["MAX_PLAYERS"] != "20" {
		t.Fatalf("unexpected response: %+v", got)
	}
	if len(got.Ports) != 1 || got.Ports[0].Host != 25565 {
		t.Fatalf("unexpected ports: %+v", got.Ports)
	}
}

func TestGetConfig_InvalidContainerID(t *testing.T) {
	router := newConfigTestRouter(&mockConfigurator{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/instances/%20/config", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateConfig_Success(t *testing.T) {
	router := newConfigTestRouter(&mockConfigurator{
		updateFn: func(_ context.Context, containerID string, params map[string]string, ports []model.PortMapping) (string, error) {
			if containerID != "c1" {
				t.Fatalf("unexpected container id: %s", containerID)
			}
			if params["MAX_PLAYERS"] != "50" {
				t.Fatalf("unexpected params: %+v", params)
			}
			if len(ports) != 1 || ports[0].Host != 25575 || ports[0].Container != 25565 {
				t.Fatalf("unexpected ports: %+v", ports)
			}
			return "c2", nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/instances/c1/config", strings.NewReader(`{"params":{"MAX_PLAYERS":"50"},"ports":[{"host":25575,"container":25565,"protocol":"tcp"}]}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got updateConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "success" || got.ContainerID != "c2" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestUpdateConfig_InvalidJSON(t *testing.T) {
	router := newConfigTestRouter(&mockConfigurator{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/instances/c1/config", strings.NewReader("{bad"))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateConfig_ContainerNotStopped(t *testing.T) {
	router := newConfigTestRouter(&mockConfigurator{
		updateFn: func(_ context.Context, _ string, _ map[string]string, _ []model.PortMapping) (string, error) {
			return "", model.ErrContainerNotStopped
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/instances/c1/config", strings.NewReader(`{"params":{}}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
