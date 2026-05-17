package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// NewRouter 注册 API 路由并包装中间件。
func NewRouter(
	h *Handler,
	games *GameHandler,
	ws *WsHandler,
	console *ConsoleHandler,
	config *ConfigHandler,
	files *FilesHandler,
	monitor *MonitorHandler,
	containerStats *ContainerStatsHandler,
	minecraftVersions *MinecraftVersionHandler,
	systemLogs ...*SystemLogHandler,
) http.Handler {
	mux := http.NewServeMux()
	var systemLogHandler *SystemLogHandler
	if len(systemLogs) > 0 {
		systemLogHandler = systemLogs[0]
	}

	mux.HandleFunc("GET /api/instances", h.GetInstances)
	mux.HandleFunc("POST /api/instances", h.CreateInstance)
	mux.HandleFunc("POST /api/instances/{id}/start", h.StartInstance)
	mux.HandleFunc("POST /api/instances/{id}/stop", h.StopInstance)
	mux.HandleFunc("POST /api/instances/{id}/restart", h.RestartInstance)
	mux.HandleFunc("POST /api/instances/{id}/force-stop", h.ForceStopInstance)
	mux.HandleFunc("DELETE /api/instances/{id}", h.DeleteInstance)
	mux.HandleFunc("DELETE /api/instances/{id}/force-delete", h.ForceDeleteInstance)
	if config != nil {
		mux.HandleFunc("GET /api/instances/{id}/config", config.HandleGetConfig)
		mux.HandleFunc("PUT /api/instances/{id}/config", config.HandleUpdateConfig)
	}
	if files != nil {
		mux.HandleFunc("GET /api/instances/{id}/files/mounts", files.HandleMounts)
		mux.HandleFunc("GET /api/instances/{id}/files", files.HandleList)
		mux.HandleFunc("POST /api/instances/{id}/files/dir", files.HandleCreateDir)
		mux.HandleFunc("POST /api/instances/{id}/files/upload", files.HandleUpload)
		mux.HandleFunc("GET /api/instances/{id}/files/download", files.HandleDownload)
		mux.HandleFunc("DELETE /api/instances/{id}/files", files.HandleDelete)
		mux.HandleFunc("GET /api/instances/{id}/files/content", files.HandleReadContent)
		mux.HandleFunc("PUT /api/instances/{id}/files/content", files.HandleWriteContent)
	}
	if monitor != nil {
		mux.HandleFunc("GET /api/monitor/server", monitor.HandleGetServerMetrics)
	}
	if containerStats != nil {
		mux.HandleFunc("GET /api/instances/{id}/stats", containerStats.HandleGetStats)
	}
	if systemLogHandler != nil {
		mux.HandleFunc("GET /api/system/logs", systemLogHandler.HandleList)
	}
	if ws != nil {
		mux.HandleFunc("GET /api/ws/events", ws.HandleEvents)
	}
	if console != nil {
		mux.HandleFunc("GET /api/ws/console/{id}", console.HandleConsole)
	}
	if games != nil {
		mux.HandleFunc("GET /api/games", games.GetGames)
		mux.HandleFunc("GET /api/games/{id}/template", games.GetGameTemplate)
	}
	if minecraftVersions != nil {
		mux.HandleFunc("GET /api/games/minecraft-java/versions", minecraftVersions.HandleMinecraftVersions)
		mux.HandleFunc("GET /api/games/minecraft-java/loader-versions", minecraftVersions.HandleMinecraftLoaderVersions)
	}

	return withRequestLogging(withCORS(mux))
}

// withCORS 添加宽松的 CORS 响应头并处理 OPTIONS 预检请求。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/ws/") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/ws/") {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		}
		switch {
		case recorder.status >= http.StatusInternalServerError:
			slog.Error("http request", attrs...)
		case recorder.status >= http.StatusBadRequest:
			slog.Warn("http request", attrs...)
		default:
			if r.Method == http.MethodGet || r.Method == http.MethodOptions {
				return
			}
			slog.Info("http request", attrs...)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
