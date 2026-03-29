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

// mockService implements InstanceService for handler tests.
type mockService struct {
	listFn   func(ctx context.Context) ([]model.Instance, error)
	createFn func(ctx context.Context, name string) (string, error)
	startFn  func(ctx context.Context, id string) error
	stopFn   func(ctx context.Context, id string) error
	deleteFn func(ctx context.Context, id string) error
}

func (m *mockService) ListInstances(ctx context.Context) ([]model.Instance, error) {
	return m.listFn(ctx)
}
func (m *mockService) CreateInstance(ctx context.Context, name string) (string, error) {
	return m.createFn(ctx, name)
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

func newTestRouter(m *mockService) http.Handler {
	h := NewHandler(m)
	return NewRouter(h)
}

// --- GET /api/instances ---

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

// --- POST /api/instances ---

func TestCreateInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, name string) (string, error) {
			if name != "test-server" {
				t.Fatalf("unexpected name: %s", name)
			}
			return "abc123", nil
		},
	})

	body := `{"name":"test-server"}`
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
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"  "}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_NameConflict(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, _ string) (string, error) {
			return "", model.ErrNameExists
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"dup"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// --- POST /api/instances/{id}/start ---

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

// --- POST /api/instances/{id}/stop ---

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

// --- DELETE /api/instances/{id} ---

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

// errTest is a generic error used in test stubs.
var errTest = errorString("test error")

type errorString string

func (e errorString) Error() string { return string(e) }
