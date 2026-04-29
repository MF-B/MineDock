package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"minedock/backend/internal/model"
)

func newTestFileService(t *testing.T, volumes []model.VolumeMount) (*FileService, string) {
	t.Helper()
	dataDir := t.TempDir()
	store := newFakeInstanceStore(model.Instance{ContainerID: "c1", Name: "server-1", GameID: "minecraft-java"})
	registry := &fakeRegistry{
		template: model.GameTemplate{
			Container: model.ContainerConfig{Volumes: volumes},
		},
	}
	return NewFileService(store, registry, dataDir), dataDir
}

func TestFileService_ListAndMounts(t *testing.T) {
	svc, dataDir := newTestFileService(t, []model.VolumeMount{{
		Name:          "server-data",
		ContainerPath: "/data",
	}})
	root := filepath.Join(dataDir, "server-1", "volumes", "server-data")
	if err := os.MkdirAll(filepath.Join(root, "world"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "world", "region"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "world", "level.dat"), []byte("level"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "world", "region", "r.0.0.mca"), []byte("chunk"), 0o644); err != nil {
		t.Fatalf("write deep file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "server.properties"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mounts, err := svc.Mounts(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Mounts: %v", err)
	}
	if len(mounts) != 1 || mounts[0].Name != "server-data" || mounts[0].ContainerPath != "/data" {
		t.Fatalf("unexpected mounts: %+v", mounts)
	}

	entries, err := svc.List(context.Background(), "c1", "server-data", "/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := []string{entries[0].Name, entries[1].Name}
	if !slices.Equal(names, []string{"world", "server.properties"}) {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if entries[0].Size != int64(len("level")+len("chunk")) {
		t.Fatalf("unexpected directory size: %+v", entries[0])
	}
	if entries[1].Size != int64(len("x")) {
		t.Fatalf("unexpected file size: %+v", entries[1])
	}
}

func TestFileService_WriteOperations(t *testing.T) {
	svc, dataDir := newTestFileService(t, []model.VolumeMount{{
		Name:          "server-data",
		ContainerPath: "/data",
	}})
	root := filepath.Join(dataDir, "server-1", "volumes", "server-data")

	if err := svc.CreateDir(context.Background(), "c1", "server-data", "/mods"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := svc.SaveUpload(context.Background(), "c1", "server-data", "/mods/a.txt", strings.NewReader("hello")); err != nil {
		t.Fatalf("SaveUpload: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "mods", "a.txt"))
	if err != nil {
		t.Fatalf("read upload: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected content: %q", content)
	}

	file, info, err := svc.OpenDownload(context.Background(), "c1", "server-data", "/mods/a.txt")
	if err != nil {
		t.Fatalf("OpenDownload: %v", err)
	}
	_ = file.Close()
	if info.Name() != "a.txt" {
		t.Fatalf("unexpected download info: %s", info.Name())
	}

	if err := svc.Delete(context.Background(), "c1", "server-data", "/mods", true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "mods")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected mods removed, got %v", err)
	}
}

func TestFileService_ReadOnlyMountRejectsWrites(t *testing.T) {
	svc, _ := newTestFileService(t, []model.VolumeMount{{
		Name:          "server-data",
		ContainerPath: "/data",
		ReadOnly:      true,
	}})

	err := svc.CreateDir(context.Background(), "c1", "server-data", "/mods")
	if !errors.Is(err, model.ErrReadOnlyMount) {
		t.Fatalf("expected ErrReadOnlyMount, got %v", err)
	}
}

func TestFileService_RejectsInvalidPathAndSymlink(t *testing.T) {
	svc, dataDir := newTestFileService(t, []model.VolumeMount{{
		Name:          "server-data",
		ContainerPath: "/data",
	}})

	_, err := svc.List(context.Background(), "c1", "server-data", "../outside")
	if !errors.Is(err, model.ErrPathInvalid) {
		t.Fatalf("expected ErrPathInvalid, got %v", err)
	}

	if runtime.GOOS == "windows" {
		t.Skip("symlink creation usually requires privileges on Windows")
	}
	root := filepath.Join(dataDir, "server-1", "volumes", "server-data")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, err = svc.List(context.Background(), "c1", "server-data", "/link")
	if !errors.Is(err, model.ErrPathInvalid) {
		t.Fatalf("expected symlink ErrPathInvalid, got %v", err)
	}
	err = svc.SaveUpload(context.Background(), "c1", "server-data", "/link", strings.NewReader("x"))
	if !errors.Is(err, model.ErrPathInvalid) {
		t.Fatalf("expected upload symlink ErrPathInvalid, got %v", err)
	}
}
