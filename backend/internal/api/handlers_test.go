package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"minedock/backend/internal/model"
)

// mockService 为 Handler 测试实现 InstanceService。
type mockService struct {
	listFn   func(ctx context.Context) ([]model.Instance, error)
	createFn func(ctx context.Context, name, imageID string) (string, error)
	startFn  func(ctx context.Context, id string) error
	stopFn   func(ctx context.Context, id string) error
	deleteFn func(ctx context.Context, id string) error
}

func (m *mockService) ListInstances(ctx context.Context) ([]model.Instance, error) {
	return m.listFn(ctx)
}
func (m *mockService) CreateInstance(ctx context.Context, name, imageID string) (string, error) {
	return m.createFn(ctx, name, imageID)
}
func (m *mockService) StartInstance(ctx context.Context, id string) error {
	return m.startFn(ctx, id)
}
func (m *mockService) StopInstance(ctx context.Context, id string) error {
	return m.stopFn(ctx, id)
}
func (m *mockService) DeleteInstance(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}

type mockRegistryLister struct {
	listFn func(ctx context.Context) []model.RegistryImage
}

func (m *mockRegistryLister) ListImages(ctx context.Context) []model.RegistryImage {
	if m == nil || m.listFn == nil {
		return []model.RegistryImage{}
	}
	return m.listFn(ctx)
}

func newTestRouter(m *mockService) http.Handler {
	h := NewHandler(m)
	rh := NewRegistryHandler(&mockRegistryLister{
		listFn: func(_ context.Context) []model.RegistryImage {
			return []model.RegistryImage{}
		},
	})
	return NewRouter(h, rh, nil)
}

// --- GET /api/instances 场景 ---

func TestGetInstances_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		listFn: func(_ context.Context) ([]model.Instance, error) {
			return []model.Instance{
				{ContainerID: "c1", Name: "server-1", Status: "Running"},
			}, nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/instances", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got []model.Instance
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ContainerID != "c1" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestGetInstances_Error(t *testing.T) {
	router := newTestRouter(&mockService{
		listFn: func(_ context.Context) ([]model.Instance, error) {
			return nil, errTest
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/instances", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetRegistryImages_Success(t *testing.T) {
	h := NewHandler(&mockService{})
	rh := NewRegistryHandler(&mockRegistryLister{
		listFn: func(_ context.Context) []model.RegistryImage {
			return []model.RegistryImage{{ID: "minecraft-java", Image: "itzg/minecraft-server:latest"}}
		},
	})
	router := NewRouter(h, rh, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/registry/images", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got []model.RegistryImage
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ID != "minecraft-java" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

// --- POST /api/instances 场景 ---

func TestCreateInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, name, imageID string) (string, error) {
			if name != "test-server" {
				t.Fatalf("unexpected name: %s", name)
			}
			if imageID != "minecraft-java" {
				t.Fatalf("unexpected image_id: %s", imageID)
			}
			return "abc123", nil
		},
	})

	body := `{"name":"test-server","image_id":"minecraft-java"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(body))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp createResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "success" || resp.ContainerID != "abc123" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestCreateInstance_InvalidJSON(t *testing.T) {
	router := newTestRouter(&mockService{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader("{bad"))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_EmptyName(t *testing.T) {
	router := newTestRouter(&mockService{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"  ","image_id":"minecraft-java"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_EmptyImageID(t *testing.T) {
	router := newTestRouter(&mockService{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"server","image_id":"   "}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_NameConflict(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, _, _ string) (string, error) {
			return "", model.ErrNameExists
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"dup","image_id":"minecraft-java"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateInstance_ImageNotFound(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, _, _ string) (string, error) {
			return "", model.ErrImageNotFound
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"dup","image_id":"not-exists"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- POST /api/instances/{id}/start 场景 ---

func TestStartInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		startFn: func(_ context.Context, id string) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances/abc123/start", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartInstance_Error(t *testing.T) {
	router := newTestRouter(&mockService{
		startFn: func(_ context.Context, _ string) error {
			return errTest
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances/abc123/start", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- POST /api/instances/{id}/stop 场景 ---

func TestStopInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		stopFn: func(_ context.Context, id string) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances/abc123/stop", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DELETE /api/instances/{id} 场景 ---

func TestDeleteInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		deleteFn: func(_ context.Context, id string) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/instances/abc123", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteInstance_Running(t *testing.T) {
	router := newTestRouter(&mockService{
		deleteFn: func(_ context.Context, _ string) error {
			return model.ErrInstanceRunning
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/instances/abc123", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// errTest 是测试桩使用的通用错误。
var errTest = errorString("test error")

type errorString string

func (e errorString) Error() string { return string(e) }
