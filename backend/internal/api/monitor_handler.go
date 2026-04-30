package api

import (
	"context"
	"net/http"

	"minedock/backend/internal/model"
)

// ServerMonitor defines host-level monitor data retrieval.
type ServerMonitor interface {
	GetServerMetrics(ctx context.Context) (*model.ServerMetrics, error)
}

// MonitorHandler exposes host resource monitoring HTTP handlers.
type MonitorHandler struct {
	monitor ServerMonitor
}

// NewMonitorHandler creates a MonitorHandler.
func NewMonitorHandler(monitor ServerMonitor) *MonitorHandler {
	return &MonitorHandler{monitor: monitor}
}

// HandleGetServerMetrics handles GET /api/monitor/server.
func (h *MonitorHandler) HandleGetServerMetrics(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.monitor == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "monitor service unavailable"})
		return
	}

	metrics, err := h.monitor.GetServerMetrics(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}
