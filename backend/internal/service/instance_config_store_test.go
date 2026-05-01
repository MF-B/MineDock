package service

import (
	"context"
	"os"
	"testing"

	"minedock/backend/internal/model"
)

func TestInstanceConfigFileStoreSaveLoad(t *testing.T) {
	store := NewInstanceConfigFileStore(t.TempDir())

	configPath, err := store.ConfigPath("server-1")
	if err != nil {
		t.Fatalf("config path: %v", err)
	}

	want := &model.StoredInstanceConfig{
		SchemaVersion: 1,
		GameID:        "minecraft-java",
		Source:        "manual",
		Image:         "itzg/minecraft-server:java8",
		Env:           map[string]string{"EULA": "TRUE"},
		GameConfig:    map[string]string{"kind": "minecraft_java"},
	}

	if err := store.Save(context.Background(), configPath, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(configPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp config should not remain, err=%v", err)
	}

	got, err := store.Load(context.Background(), configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Image != want.Image || got.Env["EULA"] != "TRUE" || got.GameConfig["kind"] != "minecraft_java" {
		t.Fatalf("unexpected config: %+v", got)
	}
}
