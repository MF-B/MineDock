package api

import (
	"context"
	"net/http"

	"minedock/backend/internal/model"
)

// ContainerStatsProvider 定义获取容器资源指标的业务接口。
type ContainerStatsProvider interface {
	GetContainerStats(ctx context.Context, containerID string) (*model.ContainerStats, error)
}

// ContainerStatsHandler 暴露容器资源监控 HTTP 处理器。
type ContainerStatsHandler struct {
	stats ContainerStatsProvider
}

// NewContainerStatsHandler 创建 ContainerStatsHandler。
func NewContainerStatsHandler(stats ContainerStatsProvider) *ContainerStatsHandler {
	return &ContainerStatsHandler{stats: stats}
}

// HandleGetStats 处理 GET /api/instances/{id}/stats。
func (h *ContainerStatsHandler) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.stats == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "container stats service unavailable"})
		return
	}

	id, ok := pathContainerID(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid container id"})
		return
	}

	stats, err := h.stats.GetContainerStats(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
