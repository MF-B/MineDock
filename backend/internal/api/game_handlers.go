package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"minedock/backend/internal/model"
)

// GameLister 定义游戏目录 Handler 依赖的查询操作。
type GameLister interface {
	ListGames(ctx context.Context) []model.Game
	GetTemplate(ctx context.Context, id string) (model.GameTemplate, error)
}

// GameHandler 暴露游戏目录与模板相关 HTTP 处理器。
type GameHandler struct {
	games GameLister
}

// NewGameHandler 创建 GameHandler。
func NewGameHandler(g GameLister) *GameHandler {
	return &GameHandler{games: g}
}

// GetGames 处理 GET /api/games，返回游戏目录列表。
func (h *GameHandler) GetGames(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.games == nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: "game service unavailable"})
		return
	}

	games := h.games.ListGames(r.Context())
	writeJSON(w, http.StatusOK, games)
}

// GetGameTemplate 处理 GET /api/games/{id}/template，按需返回模板详情。
func (h *GameHandler) GetGameTemplate(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.games == nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: "game service unavailable"})
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, statusResponse{Status: "error", Error: "invalid game id"})
		return
	}

	tpl, err := h.games.GetTemplate(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrGameNotFound):
			writeJSON(w, http.StatusNotFound, statusResponse{Status: "error", Error: err.Error()})
		case errors.Is(err, model.ErrTemplateNotFound):
			writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		case errors.Is(err, model.ErrTemplateInvalid):
			writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, statusResponse{Status: "error", Error: err.Error()})
		}
		return
	}

	writeJSON(w, http.StatusOK, tpl)
}
