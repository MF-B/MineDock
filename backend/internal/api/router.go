package api

import "net/http"

// NewRouter registers API endpoints and wraps them with HTTP middleware.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/instances", h.GetInstances)
	mux.HandleFunc("POST /api/instances", h.CreateInstance)
	mux.HandleFunc("POST /api/instances/{id}/start", h.StartInstance)
	mux.HandleFunc("POST /api/instances/{id}/stop", h.StopInstance)
	mux.HandleFunc("DELETE /api/instances/{id}", h.DeleteInstance)

	return withCORS(mux)
}

// withCORS adds permissive CORS headers and handles OPTIONS preflight requests.
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
