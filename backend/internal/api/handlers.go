package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"minedock/backend/internal/model"
)

// InstanceService 定义 API Handler 依赖的业务操作。
type InstanceService interface {
	ListInstances(ctx context.Context) ([]model.Instance, error)
	CreateInstance(ctx context.Context, name, gameID string, params map[string]string, ports []model.PortMapping) (string, error)
	StartInstance(ctx context.Context, containerID string) error
	StopInstance(ctx context.Context, containerID string) error
	DeleteInstance(ctx context.Context, containerID string) error
}

// Handler 暴露 HTTP 处理器。
type Handler struct {
	svc InstanceService
}

// NewHandler 使用给定的实例服务创建 Handler。
func NewHandler(svc InstanceService) *Handler {
	return &Handler{svc: svc}
}

// createRequest 定义创建实例请求体。
type createRequest struct {
	Name   string              `json:"name"`
	GameID string              `json:"game_id"`
	Params map[string]string   `json:"params"`
	Ports  []model.PortMapping `json:"ports"`
}

// statusResponse 定义通用状态响应体。
type statusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// createResponse 定义创建成功时的响应体。
type createResponse struct {
	Status      string `json:"status"`
	ContainerID string `json:"container_id"`
}

// GetInstances 处理 GET /api/instances 并返回所有托管实例。
func (h *Handler) GetInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := h.svc.ListInstances(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, instances)
}

// CreateInstance 处理 POST /api/instances 并创建新实例。
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
	req.GameID = strings.TrimSpace(req.GameID)
	if req.GameID == "" {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "game_id is required"})
		return
	}
	if req.Params == nil {
		req.Params = map[string]string{}
	}

	id, err := h.svc.CreateInstance(r.Context(), req.Name, req.GameID, req.Params, req.Ports)
	if err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, createResponse{Status: "success", ContainerID: id})
}

// TODO: 抽取 start/stop/delete 的公共处理流程。
// 这三个处理器都重复了：解析容器 ID、调用 Service、写回状态 JSON。
// StartInstance 处理 POST /api/instances/{id}/start 并启动实例。
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

// StopInstance 处理 POST /api/instances/{id}/stop 并停止实例。
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

// DeleteInstance 处理 DELETE /api/instances/{id} 并删除实例。
func (h *Handler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	if err := h.svc.DeleteInstance(r.Context(), id); err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{Status: "success"})
}

// pathContainerID 解析并校验 URL 路径中的容器 ID。
func pathContainerID(r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

// mapErrorCode 将领域错误统一映射为 HTTP 状态码。
func mapErrorCode(err error) int {
	switch {
	case errors.Is(err, model.ErrGameNotFound):
		return http.StatusBadRequest
	case errors.Is(err, model.ErrInvalidParams):
		return http.StatusBadRequest
	case errors.Is(err, model.ErrNameExists):
		return http.StatusConflict
	case errors.Is(err, model.ErrInstanceRunning):
		return http.StatusConflict
	case errors.Is(err, model.ErrContainerNotStopped):
		return http.StatusConflict
	case errors.Is(err, model.ErrTemplateNotFound):
		return http.StatusInternalServerError
	case errors.Is(err, model.ErrTemplateInvalid):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// writeJSON 按指定状态码写入 JSON 响应。
func writeJSON(w http.ResponseWriter, code int, v any) {
	// 说明：当前会忽略编码错误，因为状态码可能已写入。
	// TODO: 增加统一的编码错误日志，提升可观测性。
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
