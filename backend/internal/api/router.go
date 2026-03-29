package api

import "net/http"

// NewRouter 注册 API 路由并包装中间件。
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/instances", h.GetInstances)
	mux.HandleFunc("POST /api/instances", h.CreateInstance)
	mux.HandleFunc("POST /api/instances/{id}/start", h.StartInstance)
	mux.HandleFunc("POST /api/instances/{id}/stop", h.StopInstance)
	mux.HandleFunc("DELETE /api/instances/{id}", h.DeleteInstance)

	return withCORS(mux)
}

// withCORS 添加宽松的 CORS 响应头并处理 OPTIONS 预检请求。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
