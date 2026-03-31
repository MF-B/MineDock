package api

import (
	"context"
	"net/http"

	"minedock/backend/internal/model"
)

// RegistryLister 定义注册表 Handler 依赖的查询操作。
type RegistryLister interface {
	ListImages(ctx context.Context) []model.RegistryImage
}

// RegistryHandler 暴露镜像注册表相关 HTTP 处理器。
type RegistryHandler struct {
	registry RegistryLister
}

// NewRegistryHandler 创建 RegistryHandler。
func NewRegistryHandler(r RegistryLister) *RegistryHandler {
	return &RegistryHandler{registry: r}
}

// GetImages 处理 GET /api/registry/images，返回可用镜像列表。
func (h *RegistryHandler) GetImages(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: "registry service unavailable"})
		return
	}

	images := h.registry.ListImages(r.Context())
	writeJSON(w, http.StatusOK, images)
}
