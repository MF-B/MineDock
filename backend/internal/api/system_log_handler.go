package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"minedock/backend/internal/model"
	"minedock/backend/internal/service"
)

// SystemLogReader defines backend system log retrieval.
type SystemLogReader interface {
	List(ctx context.Context, query service.SystemLogQuery) (*model.SystemLogsResponse, error)
}

// SystemLogHandler exposes backend system logs over HTTP.
type SystemLogHandler struct {
	logs SystemLogReader
}

// NewSystemLogHandler creates a SystemLogHandler.
func NewSystemLogHandler(logs SystemLogReader) *SystemLogHandler {
	return &SystemLogHandler{logs: logs}
}

// HandleList handles GET /api/system/logs.
func (h *SystemLogHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.logs == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "system log service unavailable"})
		return
	}

	tail := 500
	if rawTail := strings.TrimSpace(r.URL.Query().Get("tail")); rawTail != "" {
		parsed, err := strconv.Atoi(rawTail)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid tail"})
			return
		}
		tail = parsed
	}

	resp, err := h.logs.List(r.Context(), service.SystemLogQuery{
		Tail:  tail,
		Level: r.URL.Query().Get("level"),
		Query: r.URL.Query().Get("q"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
