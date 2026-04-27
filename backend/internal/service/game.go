package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"minedock/backend/internal/model"
)

var allowedParamTypes = map[string]struct{}{
	"string":  {},
	"number":  {},
	"boolean": {},
	"select":  {},
}

// GameService 提供游戏目录查询和模板按需加载能力。
type GameService struct {
	games       []model.Game
	gameMap     map[string]model.Game
	templateDir string
}

// NewGameService 从 games.json 加载游戏目录，并记录模板目录路径。
func NewGameService(gamesFilePath string, templateDirPath string) (*GameService, error) {
	gamesPath := strings.TrimSpace(gamesFilePath)
	if gamesPath == "" {
		return nil, fmt.Errorf("games path is required")
	}

	templateDir := strings.TrimSpace(templateDirPath)
	if templateDir == "" {
		return nil, fmt.Errorf("template dir path is required")
	}

	content, err := os.ReadFile(gamesPath)
	if err != nil {
		return nil, fmt.Errorf("read games file: %w", err)
	}

	var games []model.Game
	if err := json.Unmarshal(content, &games); err != nil {
		return nil, fmt.Errorf("decode games file: %w", err)
	}

	gameMap := make(map[string]model.Game, len(games))
	for i, game := range games {
		normalized, err := normalizeGame(game)
		if err != nil {
			return nil, fmt.Errorf("invalid game at index %d: %w", i, err)
		}

		if _, exists := gameMap[normalized.ID]; exists {
			return nil, fmt.Errorf("duplicate game id: %s", normalized.ID)
		}

		templatePath := filepath.Join(templateDir, normalized.ID+".yaml")
		if _, err := os.Stat(templatePath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("template for game %q: %w", normalized.ID, model.ErrTemplateNotFound)
			}
			return nil, fmt.Errorf("stat template for game %q: %w", normalized.ID, err)
		}

		games[i] = normalized
		gameMap[normalized.ID] = normalized
	}

	return &GameService{games: games, gameMap: gameMap, templateDir: templateDir}, nil
}

// ListGames 返回游戏目录（轻量列表）。
func (s *GameService) ListGames(_ context.Context) []model.Game {
	if s == nil || len(s.games) == 0 {
		return []model.Game{}
	}
	out := make([]model.Game, len(s.games))
	copy(out, s.games)
	return out
}

// GetGame 按 ID 查找游戏条目。
func (s *GameService) GetGame(_ context.Context, id string) (model.Game, error) {
	if s == nil {
		return model.Game{}, model.ErrGameNotFound
	}

	key := strings.TrimSpace(id)
	if key == "" {
		return model.Game{}, model.ErrGameNotFound
	}

	game, ok := s.gameMap[key]
	if !ok {
		return model.Game{}, model.ErrGameNotFound
	}
	return game, nil
}

// GetTemplate 按游戏 ID 加载并返回对应的 YAML 模板。
func (s *GameService) GetTemplate(ctx context.Context, id string) (model.GameTemplate, error) {
	if s == nil {
		return model.GameTemplate{}, model.ErrGameNotFound
	}

	game, err := s.GetGame(ctx, id)
	if err != nil {
		return model.GameTemplate{}, err
	}

	templatePath := filepath.Join(s.templateDir, game.ID+".yaml")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.GameTemplate{}, model.ErrTemplateNotFound
		}
		return model.GameTemplate{}, fmt.Errorf("read template %q: %w", game.ID, err)
	}

	var tpl model.GameTemplate
	if err := yaml.Unmarshal(content, &tpl); err != nil {
		return model.GameTemplate{}, fmt.Errorf("parse template %q: %w: %v", game.ID, model.ErrTemplateInvalid, err)
	}

	if err := normalizeTemplate(game.ID, &tpl); err != nil {
		return model.GameTemplate{}, err
	}

	return tpl, nil
}

func normalizeGame(game model.Game) (model.Game, error) {
	game.ID = strings.TrimSpace(game.ID)
	game.Name = strings.TrimSpace(game.Name)
	game.Description = strings.TrimSpace(game.Description)
	game.Category = strings.TrimSpace(game.Category)
	game.Icon = strings.TrimSpace(game.Icon)

	if game.ID == "" {
		return model.Game{}, fmt.Errorf("id is required")
	}
	if game.Name == "" {
		return model.Game{}, fmt.Errorf("name is required")
	}
	if game.Description == "" {
		return model.Game{}, fmt.Errorf("description is required")
	}
	if game.Category == "" {
		return model.Game{}, fmt.Errorf("category is required")
	}
	if game.Icon == "" {
		return model.Game{}, fmt.Errorf("icon is required")
	}

	return game, nil
}

func normalizeTemplate(gameID string, tpl *model.GameTemplate) error {
	tpl.Image.Name = strings.TrimSpace(tpl.Image.Name)
	tpl.Image.Tag = strings.TrimSpace(tpl.Image.Tag)
	if tpl.Image.Name == "" {
		return fmt.Errorf("template %q image.name is required: %w", gameID, model.ErrTemplateInvalid)
	}
	if tpl.Image.Tag == "" {
		tpl.Image.Tag = "latest"
	}

	normalizedEnv := make(map[string]string, len(tpl.Container.Env))
	for key, value := range tpl.Container.Env {
		envKey := strings.TrimSpace(key)
		if envKey == "" {
			return fmt.Errorf("template %q contains empty env key: %w", gameID, model.ErrTemplateInvalid)
		}
		normalizedEnv[envKey] = value
	}
	tpl.Container.Env = normalizedEnv

	for i := range tpl.Container.Ports {
		if tpl.Container.Ports[i].Host <= 0 || tpl.Container.Ports[i].Container <= 0 {
			return fmt.Errorf("template %q port index %d is invalid: %w", gameID, i, model.ErrTemplateInvalid)
		}

		protocol := strings.ToLower(strings.TrimSpace(tpl.Container.Ports[i].Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		if protocol != "tcp" && protocol != "udp" {
			return fmt.Errorf("template %q port index %d has unsupported protocol: %w", gameID, i, model.ErrTemplateInvalid)
		}
		tpl.Container.Ports[i].Protocol = protocol
	}

	for i := range tpl.Container.Volumes {
		tpl.Container.Volumes[i].Name = strings.TrimSpace(tpl.Container.Volumes[i].Name)
		tpl.Container.Volumes[i].ContainerPath = strings.TrimSpace(tpl.Container.Volumes[i].ContainerPath)
		if tpl.Container.Volumes[i].ContainerPath == "" {
			return fmt.Errorf("template %q volume index %d container_path is required: %w", gameID, i, model.ErrTemplateInvalid)
		}
	}

	params := make([]model.TemplateParam, 0, len(tpl.Params))
	seenKeys := make(map[string]struct{}, len(tpl.Params))
	for i := range tpl.Params {
		param := tpl.Params[i]
		param.Key = strings.TrimSpace(param.Key)
		param.Label = strings.TrimSpace(param.Label)
		param.Description = strings.TrimSpace(param.Description)
		param.Type = strings.ToLower(strings.TrimSpace(param.Type))
		param.EnvVar = strings.TrimSpace(param.EnvVar)

		if param.Key == "" {
			return fmt.Errorf("template %q param index %d key is required: %w", gameID, i, model.ErrTemplateInvalid)
		}
		if _, exists := seenKeys[param.Key]; exists {
			return fmt.Errorf("template %q has duplicate param key %q: %w", gameID, param.Key, model.ErrTemplateInvalid)
		}
		seenKeys[param.Key] = struct{}{}

		if _, ok := allowedParamTypes[param.Type]; !ok {
			return fmt.Errorf("template %q param %q has unsupported type %q: %w", gameID, param.Key, param.Type, model.ErrTemplateInvalid)
		}

		if param.Label == "" {
			param.Label = param.Key
		}
		if param.EnvVar == "" {
			param.EnvVar = param.Key
		}

		if param.Type == "select" {
			if len(param.Options) == 0 {
				return fmt.Errorf("template %q param %q options are required for select type: %w", gameID, param.Key, model.ErrTemplateInvalid)
			}
			if err := validateSelectParam(gameID, param); err != nil {
				return err
			}
		}

		params = append(params, param)
	}
	if params == nil {
		params = []model.TemplateParam{}
	}
	tpl.Params = params

	if tpl.Container.Ports == nil {
		tpl.Container.Ports = []model.PortMapping{}
	}
	if tpl.Container.Volumes == nil {
		tpl.Container.Volumes = []model.VolumeMount{}
	}

	return nil
}

func validateSelectParam(gameID string, param model.TemplateParam) error {
	options := make(map[string]struct{}, len(param.Options))
	for i := range param.Options {
		value := strings.TrimSpace(param.Options[i].Value)
		label := strings.TrimSpace(param.Options[i].Label)
		if value == "" {
			return fmt.Errorf("template %q param %q option index %d value is required: %w", gameID, param.Key, i, model.ErrTemplateInvalid)
		}
		if label == "" {
			label = value
		}
		if _, exists := options[value]; exists {
			return fmt.Errorf("template %q param %q has duplicate option %q: %w", gameID, param.Key, value, model.ErrTemplateInvalid)
		}
		param.Options[i].Value = value
		param.Options[i].Label = label
		options[value] = struct{}{}
	}

	if param.Default == nil {
		return nil
	}

	defaultValue := strings.TrimSpace(fmt.Sprint(param.Default))
	if defaultValue == "" {
		return nil
	}
	if _, ok := options[defaultValue]; !ok {
		return fmt.Errorf("template %q param %q default value %q not in options: %w", gameID, param.Key, defaultValue, model.ErrTemplateInvalid)
	}

	return nil
}
