package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"minedock/backend/internal/model"
)

type mockFileManager struct {
	mountsFn   func(ctx context.Context, containerID string) ([]model.FileMount, error)
	listFn     func(ctx context.Context, containerID, mountName, path string) ([]model.FileEntry, error)
	createFn   func(ctx context.Context, containerID, mountName, path string) error
	deleteFn   func(ctx context.Context, containerID, mountName, path string, recursive bool) error
	downloadFn func(ctx context.Context, containerID, mountName, path string) (*os.File, os.FileInfo, error)
	uploadFn   func(ctx context.Context, containerID, mountName, path string, reader io.Reader) error
}

func (m *mockFileManager) Mounts(ctx context.Context, containerID string) ([]model.FileMount, error) {
	return m.mountsFn(ctx, containerID)
}

func (m *mockFileManager) List(ctx context.Context, containerID, mountName, path string) ([]model.FileEntry, error) {
	return m.listFn(ctx, containerID, mountName, path)
}

func (m *mockFileManager) CreateDir(ctx context.Context, containerID, mountName, path string) error {
	return m.createFn(ctx, containerID, mountName, path)
}

func (m *mockFileManager) Delete(ctx context.Context, containerID, mountName, path string, recursive bool) error {
	return m.deleteFn(ctx, containerID, mountName, path, recursive)
}

func (m *mockFileManager) OpenDownload(ctx context.Context, containerID, mountName, path string) (*os.File, os.FileInfo, error) {
	return m.downloadFn(ctx, containerID, mountName, path)
}

func (m *mockFileManager) SaveUpload(ctx context.Context, containerID, mountName, path string, reader io.Reader) error {
	return m.uploadFn(ctx, containerID, mountName, path, reader)
}

func newFilesTestRouter(files *mockFileManager) http.Handler {
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
	return NewRouter(h, nil, nil, nil, nil, NewFilesHandler(files), nil, nil)
}

func TestFilesMounts_Success(t *testing.T) {
	router := newFilesTestRouter(&mockFileManager{
		mountsFn: func(_ context.Context, containerID string) ([]model.FileMount, error) {
			if containerID != "c1" {
				t.Fatalf("unexpected container id: %s", containerID)
			}
			return []model.FileMount{{Name: "server-data", ContainerPath: "/data"}}, nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/instances/c1/files/mounts", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilesList_ErrorMapping(t *testing.T) {
	router := newFilesTestRouter(&mockFileManager{
		listFn: func(_ context.Context, _, _, _ string) ([]model.FileEntry, error) {
			return nil, model.ErrMountNotFound
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/instances/c1/files?mount=missing&path=/", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestFilesCreateDir_ReadOnly(t *testing.T) {
	router := newFilesTestRouter(&mockFileManager{
		createFn: func(_ context.Context, _, _, _ string) error {
			return model.ErrReadOnlyMount
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/instances/c1/files/dir", strings.NewReader(`{"mount":"server-data","path":"/mods"}`))
	router.ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
