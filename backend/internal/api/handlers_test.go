package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"minedock/backend/internal/model"
)

// mockService 为 Handler 测试实现 InstanceService。
type mockService struct {
	listFn   func(ctx context.Context) ([]model.Instance, error)
	createFn func(
		ctx context.Context,
		name, gameID string,
		params map[string]string,
		ports []model.PortMapping,
		resources *model.ResourceLimits,
	) (string, error)
	startFn       func(ctx context.Context, id string) error
	stopFn        func(ctx context.Context, id string) error
	restartFn     func(ctx context.Context, id string) error
	forceStopFn   func(ctx context.Context, id string) error
	deleteFn      func(ctx context.Context, id string, purgeData bool) error
	forceDeleteFn func(ctx context.Context, id string, purgeData bool) error
}

func (m *mockService) ListInstances(ctx context.Context) ([]model.Instance, error) {
	return m.listFn(ctx)
}
func (m *mockService) CreateInstance(
	ctx context.Context,
	name, gameID string,
	params map[string]string,
	ports []model.PortMapping,
	resources *model.ResourceLimits,
) (string, error) {
	return m.createFn(ctx, name, gameID, params, ports, resources)
}
func (m *mockService) StartInstance(ctx context.Context, id string) error {
	return m.startFn(ctx, id)
}
func (m *mockService) StopInstance(ctx context.Context, id string) error {
	return m.stopFn(ctx, id)
}
func (m *mockService) RestartInstance(ctx context.Context, id string) error {
	return m.restartFn(ctx, id)
}
func (m *mockService) ForceStopInstance(ctx context.Context, id string) error {
	return m.forceStopFn(ctx, id)
}
func (m *mockService) DeleteInstance(ctx context.Context, id string, purgeData bool) error {
	return m.deleteFn(ctx, id, purgeData)
}
func (m *mockService) ForceDeleteInstance(ctx context.Context, id string, purgeData bool) error {
	return m.forceDeleteFn(ctx, id, purgeData)
}

type mockRegistryLister struct {
	listFn     func(ctx context.Context) []model.Game
	getTplByID func(ctx context.Context, id string) (model.GameTemplate, error)
}

func (m *mockRegistryLister) ListGames(ctx context.Context) []model.Game {
	if m == nil || m.listFn == nil {
		return []model.Game{}
	}
	return m.listFn(ctx)
}

func (m *mockRegistryLister) GetTemplate(ctx context.Context, id string) (model.GameTemplate, error) {
	if m == nil || m.getTplByID == nil {
		return model.GameTemplate{}, model.ErrGameNotFound
	}
	return m.getTplByID(ctx, id)
}

func newTestRouter(m *mockService) http.Handler {
	h := NewHandler(m)
	gh := NewGameHandler(&mockRegistryLister{
		listFn: func(_ context.Context) []model.Game {
			return []model.Game{}
		},
		getTplByID: func(_ context.Context, _ string) (model.GameTemplate, error) {
			return model.GameTemplate{}, model.ErrGameNotFound
		},
	})
	return NewRouter(h, gh, nil, nil, nil, nil, nil, nil, nil)
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

func TestGetGames_Success(t *testing.T) {
	h := NewHandler(&mockService{})
	gh := NewGameHandler(&mockRegistryLister{
		listFn: func(_ context.Context) []model.Game {
			return []model.Game{{ID: "minecraft-java", Name: "Minecraft Java"}}
		},
		getTplByID: func(_ context.Context, _ string) (model.GameTemplate, error) {
			return model.GameTemplate{}, model.ErrGameNotFound
		},
	})
	router := NewRouter(h, gh, nil, nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/games", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got []model.Game
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ID != "minecraft-java" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestGetGameTemplate_Success(t *testing.T) {
	h := NewHandler(&mockService{})
	gh := NewGameHandler(&mockRegistryLister{
		listFn: func(_ context.Context) []model.Game {
			return []model.Game{{ID: "minecraft-java", Name: "Minecraft Java"}}
		},
		getTplByID: func(_ context.Context, id string) (model.GameTemplate, error) {
			if id != "minecraft-java" {
				return model.GameTemplate{}, model.ErrGameNotFound
			}
			return model.GameTemplate{Image: model.TemplateImage{Name: "itzg/minecraft-server", Tag: "latest"}}, nil
		},
	})
	router := NewRouter(h, gh, nil, nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/games/minecraft-java/template", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got model.GameTemplate
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Image.FullImageRef() != "itzg/minecraft-server:latest" {
		t.Fatalf("unexpected template response: %+v", got)
	}
}

func TestGetGameTemplate_NotFound(t *testing.T) {
	h := NewHandler(&mockService{})
	gh := NewGameHandler(&mockRegistryLister{
		listFn: func(_ context.Context) []model.Game {
			return []model.Game{}
		},
		getTplByID: func(_ context.Context, _ string) (model.GameTemplate, error) {
			return model.GameTemplate{}, model.ErrGameNotFound
		},
	})
	router := NewRouter(h, gh, nil, nil, nil, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/games/not-exists/template", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/instances 场景 ---

func TestCreateInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(
			_ context.Context,
			name, gameID string,
			params map[string]string,
			ports []model.PortMapping,
			resources *model.ResourceLimits,
		) (string, error) {
			if name != "test-server" {
				t.Fatalf("unexpected name: %s", name)
			}
			if gameID != "minecraft-java" {
				t.Fatalf("unexpected game_id: %s", gameID)
			}
			if params["SERVER_TYPE"] != "PAPER" {
				t.Fatalf("unexpected params: %+v", params)
			}
			if len(ports) != 1 || ports[0].Host != 25575 || ports[0].Container != 25565 || ports[0].Protocol != "tcp" {
				t.Fatalf("unexpected ports: %+v", ports)
			}
			if resources != nil {
				t.Fatalf("expected nil resources, got %+v", resources)
			}
			return "abc123", nil
		},
	})

	body := `{"name":"test-server","game_id":"minecraft-java","params":{"SERVER_TYPE":"PAPER"},"ports":[{"host":25575,"container":25565,"protocol":"tcp"}]}`
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
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"  ","game_id":"minecraft-java"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_EmptyGameID(t *testing.T) {
	router := newTestRouter(&mockService{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"server","game_id":"   "}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_NameConflict(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, _, _ string, _ map[string]string, _ []model.PortMapping, _ *model.ResourceLimits) (string, error) {
			return "", model.ErrNameExists
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"dup","game_id":"minecraft-java"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateInstance_GameNotFound(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, _, _ string, _ map[string]string, _ []model.PortMapping, _ *model.ResourceLimits) (string, error) {
			return "", model.ErrGameNotFound
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"dup","game_id":"not-exists"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_InvalidParams(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, _, _ string, _ map[string]string, _ []model.PortMapping, _ *model.ResourceLimits) (string, error) {
			return "", fmtWrap(model.ErrInvalidParams)
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(`{"name":"dup","game_id":"minecraft-java","params":{"bad":"1"}}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInstance_WithResources(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(
			_ context.Context,
			_ string,
			_ string,
			_ map[string]string,
			_ []model.PortMapping,
			resources *model.ResourceLimits,
		) (string, error) {
			if resources == nil {
				t.Fatal("expected resources")
			}
			if resources.Memory != "2g" || resources.CPU != 2 {
				t.Fatalf("unexpected resources: %+v", resources)
			}
			return "abc123", nil
		},
	})

	body := `{"name":"test-server","game_id":"minecraft-java","resources":{"memory":"2g","cpu":2}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(body))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateInstance_InvalidResourceLimits(t *testing.T) {
	router := newTestRouter(&mockService{
		createFn: func(_ context.Context, _, _ string, _ map[string]string, _ []model.PortMapping, _ *model.ResourceLimits) (string, error) {
			return "", model.ErrInvalidResourceLimits
		},
	})

	body := `{"name":"test-server","game_id":"minecraft-java","resources":{"memory":"bad","cpu":0}}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances", strings.NewReader(body))
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

// --- POST /api/instances/{id}/restart 场景 ---

func TestRestartInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		restartFn: func(_ context.Context, id string) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances/abc123/restart", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRestartInstance_NotRunning(t *testing.T) {
	router := newTestRouter(&mockService{
		restartFn: func(_ context.Context, _ string) error {
			return model.ErrInstanceNotRunning
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances/abc123/restart", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// --- POST /api/instances/{id}/force-stop 场景 ---

func TestForceStopInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		forceStopFn: func(_ context.Context, id string) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances/abc123/force-stop", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DELETE /api/instances/{id} 场景 ---

func TestDeleteInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		deleteFn: func(_ context.Context, id string, purgeData bool) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			if purgeData {
				t.Fatal("expected purgeData=false")
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

func TestDeleteInstance_PurgeData(t *testing.T) {
	router := newTestRouter(&mockService{
		deleteFn: func(_ context.Context, id string, purgeData bool) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			if !purgeData {
				t.Fatal("expected purgeData=true")
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/instances/abc123?purge_data=true", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteInstance_Running(t *testing.T) {
	router := newTestRouter(&mockService{
		deleteFn: func(_ context.Context, _ string, _ bool) error {
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

// --- DELETE /api/instances/{id}/force-delete 场景 ---

func TestForceDeleteInstance_Success(t *testing.T) {
	router := newTestRouter(&mockService{
		forceDeleteFn: func(_ context.Context, id string, purgeData bool) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			if purgeData {
				t.Fatal("expected purgeData=false")
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/instances/abc123/force-delete", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestForceDeleteInstance_PurgeData(t *testing.T) {
	router := newTestRouter(&mockService{
		forceDeleteFn: func(_ context.Context, id string, purgeData bool) error {
			if id != "abc123" {
				t.Fatalf("unexpected id: %s", id)
			}
			if !purgeData {
				t.Fatal("expected purgeData=true")
			}
			return nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/instances/abc123/force-delete?purge_data=true", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// errTest 是测试桩使用的通用错误。
var errTest = errorString("test error")

type errorString string

func (e errorString) Error() string { return string(e) }

func fmtWrap(err error) error {
	if err == nil {
		return nil
	}
	return wrappedError{err: err}
}

type wrappedError struct {
	err error
}

func (e wrappedError) Error() string { return "wrapped: " + e.err.Error() }

func (e wrappedError) Unwrap() error { return e.err }

func TestMapErrorCode_InvalidParamsWrapped(t *testing.T) {
	if got := mapErrorCode(fmtWrap(model.ErrInvalidParams)); got != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrapped invalid params, got %d", got)
	}
	if !errors.Is(fmtWrap(model.ErrInvalidParams), model.ErrInvalidParams) {
		t.Fatal("expected wrapped error to match ErrInvalidParams")
	}
}

func TestMapErrorCode_ContainerNotStopped(t *testing.T) {
	if got := mapErrorCode(model.ErrContainerNotStopped); got != http.StatusConflict {
		t.Fatalf("expected 409 for container-not-stopped, got %d", got)
	}
}

func TestMapErrorCode_InstanceNotRunning(t *testing.T) {
	if got := mapErrorCode(model.ErrInstanceNotRunning); got != http.StatusConflict {
		t.Fatalf("expected 409 for instance-not-running, got %d", got)
	}
}
