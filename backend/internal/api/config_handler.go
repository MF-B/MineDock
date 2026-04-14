package api

import (
	"context"
	"encoding/json"
	"net/http"

	"minedock/backend/internal/model"
	"minedock/backend/internal/service"
)

// InstanceConfigurator 定义配置 Handler 依赖的业务能力。
type InstanceConfigurator interface {
	GetInstanceConfig(ctx context.Context, containerID string) (*service.InstanceConfig, error)
	UpdateInstanceConfig(
		ctx context.Context,
		containerID string,
		params map[string]string,
		ports []model.PortMapping,
		resources *model.ResourceLimits,
	) (string, error)
}

// ConfigHandler 暴露容器配置相关 HTTP 处理器。
type ConfigHandler struct {
	cfg InstanceConfigurator
}

// NewConfigHandler 创建 ConfigHandler。
func NewConfigHandler(cfg InstanceConfigurator) *ConfigHandler {
	return &ConfigHandler{cfg: cfg}
}

type updateConfigRequest struct {
	Params    map[string]string     `json:"params"`
	Ports     []model.PortMapping   `json:"ports"`
	Resources *model.ResourceLimits `json:"resources,omitempty"`
}

type updateConfigResponse struct {
	Status      string `json:"status"`
	ContainerID string `json:"container_id"`
}

// HandleGetConfig 处理 GET /api/instances/{id}/config。
func (h *ConfigHandler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.cfg == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "config service unavailable"})
		return
	}

	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	cfg, err := h.cfg.GetInstanceConfig(r.Context(), id)
	if err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

// HandleUpdateConfig 处理 PUT /api/instances/{id}/config。
func (h *ConfigHandler) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.cfg == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "config service unavailable"})
		return
	}

	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	var req updateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid json body"})
		return
	}
	if req.Params == nil {
		req.Params = map[string]string{}
	}

	newID, err := h.cfg.UpdateInstanceConfig(r.Context(), id, req.Params, req.Ports, req.Resources)
	if err != nil {
		writeJSON(w, mapErrorCode(err), statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, updateConfigResponse{Status: "success", ContainerID: newID})
}
