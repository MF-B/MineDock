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

func NewHandler(svc *service.DockerService) *Handler {
	return &Handler{svc: svc}
}

type createRequest struct {
	Name string `json:"name"`
}

type statusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type createResponse struct {
	Status      string `json:"status"`
	ContainerID string `json:"container_id"`
}

func (h *Handler) GetInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := h.svc.ListInstances(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, instances)
}

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
		if err == store.ErrNameExists {
			code = http.StatusConflict
		}
		writeJSON(w, code, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, createResponse{Status: "success", ContainerID: id})
}

func (h *Handler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/instances/")
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	if err := h.svc.DeleteInstance(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
