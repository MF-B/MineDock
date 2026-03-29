package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"minedock/backend/internal/service"
	"minedock/backend/internal/store"
)

// Handler exposes HTTP handlers.
type Handler struct {
	svc *service.DockerService
}

// NewHandler creates a Handler with the provided Docker service.
func NewHandler(svc *service.DockerService) *Handler {
	return &Handler{svc: svc}
}

// createRequest defines the payload for creating an instance.
type createRequest struct {
	Name string `json:"name"`
}

// statusResponse defines the standard status response body.
type statusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// createResponse defines the response body of a successful create operation.
type createResponse struct {
	Status      string `json:"status"`
	ContainerID string `json:"container_id"`
}

// GetInstances handles GET /api/instances and returns all managed instances.
func (h *Handler) GetInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := h.svc.ListInstances(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, instances)
}

// CreateInstance handles POST /api/instances and creates a new managed instance.
func (h *Handler) CreateInstance(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid json body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "name is required"})
		return
	}

	id, err := h.svc.CreateInstance(r.Context(), req.Name)
	if err != nil {
		code := http.StatusInternalServerError
		// NOTE: Keep domain error to HTTP status mapping consistent across handlers.
		if err == store.ErrNameExists {
			code = http.StatusConflict
		}
		writeJSON(w, code, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, createResponse{Status: "success", ContainerID: id})
}

// TODO(minedock): Extract a shared handler flow for start/stop/delete.
// The three handlers repeat: parse container id, call service, and write status JSON.
// StartInstance handles POST /api/instances/{id}/start and starts one instance.
func (h *Handler) StartInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	if err := h.svc.StartInstance(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}

// StopInstance handles POST /api/instances/{id}/stop and stops one instance.
func (h *Handler) StopInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	if err := h.svc.StopInstance(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}

// DeleteInstance handles DELETE /api/instances/{id} and removes one instance.
func (h *Handler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	if err := h.svc.DeleteInstance(r.Context(), id); err != nil {
		code := http.StatusInternalServerError
		// NOTE: Keep domain error to HTTP status mapping consistent across handlers.
		if err == service.ErrInstanceRunning {
			code = http.StatusConflict
		}
		writeJSON(w, code, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}

// pathContainerID parses and validates the container id from URL path params.
func pathContainerID(r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

// writeJSON writes a JSON response with the provided status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	// NOTE: Encoding errors are currently ignored because status may already be written.
	// TODO(minedock): Add centralized encoding-error logging for better observability.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
