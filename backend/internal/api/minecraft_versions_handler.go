package api

import (
	"context"
	"net/http"
	"strings"

	"minedock/backend/internal/service"
)

// MinecraftVersionProvider defines Minecraft version metadata dependencies.
type MinecraftVersionProvider interface {
	MinecraftVersions(ctx context.Context) ([]service.MinecraftVersionOption, error)
	LoaderVersions(ctx context.Context, mcVersion string, serverType string) ([]service.MinecraftLoaderVersionOption, error)
}

// MinecraftVersionHandler exposes Minecraft Java version metadata endpoints.
type MinecraftVersionHandler struct {
	versions MinecraftVersionProvider
}

// NewMinecraftVersionHandler creates a MinecraftVersionHandler.
func NewMinecraftVersionHandler(versions MinecraftVersionProvider) *MinecraftVersionHandler {
	return &MinecraftVersionHandler{versions: versions}
}

// HandleMinecraftVersions handles GET /api/games/minecraft-java/versions.
func (h *MinecraftVersionHandler) HandleMinecraftVersions(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.versions == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "version service unavailable"})
		return
	}

	versions, err := h.versions.MinecraftVersions(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// HandleMinecraftLoaderVersions handles GET /api/games/minecraft-java/loader-versions.
func (h *MinecraftVersionHandler) HandleMinecraftLoaderVersions(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.versions == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{Status: "error", Error: "version service unavailable"})
		return
	}

	mcVersion := strings.TrimSpace(r.URL.Query().Get("mc_version"))
	serverType := strings.TrimSpace(r.URL.Query().Get("server_type"))
	if mcVersion == "" || serverType == "" {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "mc_version and server_type are required"})
		return
	}

	versions, err := h.versions.LoaderVersions(r.Context(), mcVersion, serverType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, versions)
}
