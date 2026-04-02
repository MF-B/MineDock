package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"minedock/backend/internal/model"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func TestNewGameService_ListGamesSuccess(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	if err := os.Mkdir(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	writeFile(t, filepath.Join(tempDir, "games.json"), `[
  {
    "id": "minecraft-java",
    "name": "Minecraft Java",
    "description": "desc",
    "category": "minecraft",
    "icon": "minecraft-java"
  }
]`)
	writeFile(t, filepath.Join(templatesDir, "minecraft-java.yaml"), `image:
  name: "itzg/minecraft-server"
`)

	svc, err := NewGameService(filepath.Join(tempDir, "games.json"), templatesDir)
	if err != nil {
		t.Fatalf("new game service: %v", err)
	}

	games := svc.ListGames(context.Background())
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if games[0].ID != "minecraft-java" {
		t.Fatalf("unexpected game id: %s", games[0].ID)
	}

	games[0].Name = "changed"
	games2 := svc.ListGames(context.Background())
	if games2[0].Name != "Minecraft Java" {
		t.Fatalf("expected cloned result, got %s", games2[0].Name)
	}
}

func TestNewGameService_EmptyAndInvalidGamesFile(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		tempDir := t.TempDir()
		templatesDir := filepath.Join(tempDir, "templates")
		if err := os.Mkdir(templatesDir, 0o755); err != nil {
			t.Fatalf("mkdir templates: %v", err)
		}
		writeFile(t, filepath.Join(tempDir, "games.json"), `[]`)

		svc, err := NewGameService(filepath.Join(tempDir, "games.json"), templatesDir)
		if err != nil {
			t.Fatalf("new game service: %v", err)
		}
		if got := svc.ListGames(context.Background()); len(got) != 0 {
			t.Fatalf("expected empty list, got %d", len(got))
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		tempDir := t.TempDir()
		templatesDir := filepath.Join(tempDir, "templates")
		if err := os.Mkdir(templatesDir, 0o755); err != nil {
			t.Fatalf("mkdir templates: %v", err)
		}
		writeFile(t, filepath.Join(tempDir, "games.json"), `{`)

		if _, err := NewGameService(filepath.Join(tempDir, "games.json"), templatesDir); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNewGameService_MissingTemplateOnStartup(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	if err := os.Mkdir(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	writeFile(t, filepath.Join(tempDir, "games.json"), `[
  {
    "id": "minecraft-java",
    "name": "Minecraft Java",
    "description": "desc",
    "category": "minecraft",
    "icon": "minecraft-java"
  }
]`)

	_, err := NewGameService(filepath.Join(tempDir, "games.json"), templatesDir)
	if !errors.Is(err, model.ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestGameService_GetTemplateSuccess(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	if err := os.Mkdir(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	writeFile(t, filepath.Join(tempDir, "games.json"), `[
  {
    "id": "minecraft-java",
    "name": "Minecraft Java",
    "description": "desc",
    "category": "minecraft",
    "icon": "minecraft-java"
  }
]`)
	writeFile(t, filepath.Join(templatesDir, "minecraft-java.yaml"), `image:
  name: "itzg/minecraft-server"
container:
  ports:
    - host: 25565
      container: 25565
  env:
    EULA: "TRUE"
params:
  - key: "SERVER_TYPE"
    type: "select"
    default: "PAPER"
    options:
      - value: "PAPER"
`)

	svc, err := NewGameService(filepath.Join(tempDir, "games.json"), templatesDir)
	if err != nil {
		t.Fatalf("new game service: %v", err)
	}

	tpl, err := svc.GetTemplate(context.Background(), "minecraft-java")
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if tpl.Image.FullImageRef() != "itzg/minecraft-server:latest" {
		t.Fatalf("unexpected image ref: %s", tpl.Image.FullImageRef())
	}
	if len(tpl.Container.Ports) != 1 || tpl.Container.Ports[0].Protocol != "tcp" {
		t.Fatalf("unexpected port protocol: %+v", tpl.Container.Ports)
	}
	if len(tpl.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(tpl.Params))
	}
	if tpl.Params[0].EnvVar != "SERVER_TYPE" {
		t.Fatalf("expected default env var, got %s", tpl.Params[0].EnvVar)
	}
	if tpl.Params[0].Label != "SERVER_TYPE" {
		t.Fatalf("expected default label, got %s", tpl.Params[0].Label)
	}
}

func TestGameService_GetTemplateErrors(t *testing.T) {
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	if err := os.Mkdir(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	gamesPath := filepath.Join(tempDir, "games.json")
	templatePath := filepath.Join(templatesDir, "minecraft-java.yaml")
	writeFile(t, gamesPath, `[
  {
    "id": "minecraft-java",
    "name": "Minecraft Java",
    "description": "desc",
    "category": "minecraft",
    "icon": "minecraft-java"
  }
]`)
	writeFile(t, templatePath, `image:
  name: "itzg/minecraft-server"
`)

	svc, err := NewGameService(gamesPath, templatesDir)
	if err != nil {
		t.Fatalf("new game service: %v", err)
	}

	if _, err := svc.GetTemplate(context.Background(), "not-exists"); !errors.Is(err, model.ErrGameNotFound) {
		t.Fatalf("expected ErrGameNotFound, got %v", err)
	}

	if err := os.Remove(templatePath); err != nil {
		t.Fatalf("remove template: %v", err)
	}
	if _, err := svc.GetTemplate(context.Background(), "minecraft-java"); !errors.Is(err, model.ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}

	writeFile(t, templatePath, `[`) // invalid yaml
	if _, err := svc.GetTemplate(context.Background(), "minecraft-java"); !errors.Is(err, model.ErrTemplateInvalid) {
		t.Fatalf("expected ErrTemplateInvalid for yaml parse error, got %v", err)
	}

	writeFile(t, templatePath, `image:
  tag: "latest"
`)
	if _, err := svc.GetTemplate(context.Background(), "minecraft-java"); !errors.Is(err, model.ErrTemplateInvalid) {
		t.Fatalf("expected ErrTemplateInvalid for missing image name, got %v", err)
	}
}
