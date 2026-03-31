package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"minedock/backend/internal/model"
)

func writeRegistryFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	return path
}

func TestNewRegistryService_Success(t *testing.T) {
	path := writeRegistryFile(t, `[
  {
    "id": "minecraft-java",
    "name": "Minecraft Java Edition",
    "image": "itzg/minecraft-server:latest",
    "description": "desc",
    "category": "minecraft",
    "icon": "minecraft-java",
    "default_env": {"EULA": "TRUE"},
    "default_ports": ["25565:25565"]
  }
]`)

	svc, err := NewRegistryService(path)
	if err != nil {
		t.Fatalf("new registry service: %v", err)
	}

	images := svc.ListImages(context.Background())
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].ID != "minecraft-java" {
		t.Fatalf("unexpected image id: %s", images[0].ID)
	}

	images[0].DefaultEnv["EULA"] = "FALSE"
	images2 := svc.ListImages(context.Background())
	if images2[0].DefaultEnv["EULA"] != "TRUE" {
		t.Fatalf("expected cloned env map, got %s", images2[0].DefaultEnv["EULA"])
	}
}

func TestNewRegistryService_InvalidJSON(t *testing.T) {
	path := writeRegistryFile(t, `{`)

	_, err := NewRegistryService(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewRegistryService_DuplicateID(t *testing.T) {
	path := writeRegistryFile(t, `[
  {"id": "a", "name": "A", "image": "repo/a:latest", "default_env": {}, "default_ports": []},
  {"id": "a", "name": "B", "image": "repo/b:latest", "default_env": {}, "default_ports": []}
]`)

	_, err := NewRegistryService(path)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestNewRegistryService_EmptyRequiredField(t *testing.T) {
	path := writeRegistryFile(t, `[
  {"id": "", "name": "A", "image": "repo/a:latest", "default_env": {}, "default_ports": []}
]`)

	_, err := NewRegistryService(path)
	if err == nil {
		t.Fatal("expected empty id error")
	}

	path = writeRegistryFile(t, `[
  {"id": "a", "name": "A", "image": "", "default_env": {}, "default_ports": []}
]`)

	_, err = NewRegistryService(path)
	if err == nil {
		t.Fatal("expected empty image error")
	}
}

func TestRegistryService_GetImage(t *testing.T) {
	path := writeRegistryFile(t, `[
  {"id": "minecraft-java", "name": "A", "image": "repo/a:latest", "default_env": {}, "default_ports": []}
]`)

	svc, err := NewRegistryService(path)
	if err != nil {
		t.Fatalf("new registry service: %v", err)
	}

	img, err := svc.GetImage(context.Background(), "minecraft-java")
	if err != nil {
		t.Fatalf("get image: %v", err)
	}
	if img.Image != "repo/a:latest" {
		t.Fatalf("unexpected image: %s", img.Image)
	}

	_, err = svc.GetImage(context.Background(), "not-exists")
	if !errors.Is(err, model.ErrImageNotFound) {
		t.Fatalf("expected ErrImageNotFound, got %v", err)
	}
}
