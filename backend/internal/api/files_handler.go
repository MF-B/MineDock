package api

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"minedock/backend/internal/model"
	"minedock/backend/internal/service"
)

// InstanceFileManager 定义文件管理 Handler 依赖的业务能力。
type InstanceFileManager interface {
	Mounts(ctx context.Context, containerID string) ([]model.FileMount, error)
	List(ctx context.Context, containerID, mountName, path string) ([]model.FileEntry, error)
	CreateDir(ctx context.Context, containerID, mountName, path string) error
	Delete(ctx context.Context, containerID, mountName, path string, recursive bool) error
	OpenDownload(ctx context.Context, containerID, mountName, path string) (*os.File, os.FileInfo, error)
	SaveUpload(ctx context.Context, containerID, mountName, path string, reader io.Reader) error
}

// FilesHandler 暴露实例文件管理 HTTP 处理器。
type FilesHandler struct {
	files InstanceFileManager
}

// NewFilesHandler 创建 FilesHandler。
func NewFilesHandler(files InstanceFileManager) *FilesHandler {
	return &FilesHandler{files: files}
}

type createDirRequest struct {
	Mount string `json:"mount"`
	Path  string `json:"path"`
}

// HandleMounts 处理 GET /api/instances/{id}/files/mounts。
func (h *FilesHandler) HandleMounts(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.files == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "file service unavailable"})
		return
	}
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}
	mounts, err := h.files.Mounts(r.Context(), id)
	if err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, mounts)
}

// HandleList 处理 GET /api/instances/{id}/files。
func (h *FilesHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.files == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "file service unavailable"})
		return
	}
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}
	entries, err := h.files.List(r.Context(), id, r.URL.Query().Get("mount"), r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// HandleCreateDir 处理 POST /api/instances/{id}/files/dir。
func (h *FilesHandler) HandleCreateDir(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.files == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "file service unavailable"})
		return
	}
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}
	var req createDirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid json body"})
		return
	}
	if err := h.files.CreateDir(r.Context(), id, req.Mount, req.Path); err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}

// HandleUpload 处理 POST /api/instances/{id}/files/upload。
func (h *FilesHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.files == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "file service unavailable"})
		return
	}
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, service.MaxUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(service.MaxUploadBytes + 1024*1024); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid upload body"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "file is required"})
		return
	}
	defer file.Close()

	filename := strings.TrimSpace(header.Filename)
	if filename == "" || filename != path.Base(filename) {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: model.ErrPathInvalid.Error()})
		return
	}
	targetPath := path.Join(r.FormValue("path"), filename)
	if r.FormValue("path") == "" || r.FormValue("path") == "/" {
		targetPath = filename
	}
	if err := h.files.SaveUpload(r.Context(), id, r.FormValue("mount"), targetPath, file); err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}

// HandleDownload 处理 GET /api/instances/{id}/files/download。
func (h *FilesHandler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.files == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "file service unavailable"})
		return
	}
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}
	file, info, err := h.files.OpenDownload(r.Context(), id, r.URL.Query().Get("mount"), r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// HandleDelete 处理 DELETE /api/instances/{id}/files。
func (h *FilesHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.files == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "file service unavailable"})
		return
	}
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}
	recursive := false
	if raw := strings.TrimSpace(r.URL.Query().Get("recursive")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid recursive"})
			return
		}
		recursive = parsed
	}
	if err := h.files.Delete(r.Context(), id, r.URL.Query().Get("mount"), r.URL.Query().Get("path"), recursive); err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}
