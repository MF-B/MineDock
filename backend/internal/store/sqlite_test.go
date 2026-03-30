package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"minedock/backend/internal/model"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("new test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewSQLiteStore_EmptyPath(t *testing.T) {
	_, err := NewSQLiteStore("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestSaveAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	inst := model.Instance{ContainerID: "c1", Name: "server-1", Status: "Stopped"}
	if err := s.Save(ctx, inst); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok, err := s.Get(ctx, "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatal("expected instance to exist")
	}
	if got.ContainerID != "c1" || got.Name != "server-1" || got.Status != "Stopped" {
		t.Fatalf("unexpected instance: %+v", got)
	}
}

func TestSave_Upsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	inst := model.Instance{ContainerID: "c1", Name: "server-1", Status: "Stopped"}
	if err := s.Save(ctx, inst); err != nil {
		t.Fatalf("first save: %v", err)
	}

	inst.Status = "Running"
	if err := s.Save(ctx, inst); err != nil {
		t.Fatalf("upsert save: %v", err)
	}

	got, _, err := s.Get(ctx, "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "Running" {
		t.Fatalf("expected Running, got %s", got.Status)
	}
}

func TestSave_DuplicateName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Save(ctx, model.Instance{ContainerID: "c1", Name: "dup", Status: "Stopped"}); err != nil {
		t.Fatalf("first save: %v", err)
	}

	err := s.Save(ctx, model.Instance{ContainerID: "c2", Name: "dup", Status: "Stopped"})
	if !errors.Is(err, model.ErrNameExists) {
		t.Fatalf("expected ErrNameExists, got %v", err)
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, ok, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	inst := model.Instance{ContainerID: "c1", Name: "server-1", Status: "Stopped"}
	if err := s.Save(ctx, inst); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := s.Delete(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, ok, err := s.Get(ctx, "c1")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if ok {
		t.Fatal("expected instance to be deleted")
	}
}

func TestDelete_NonExistent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 删除不存在的记录不应返回错误。
	if err := s.Delete(ctx, "nonexistent"); err != nil {
		t.Fatalf("delete non-existent: %v", err)
	}
}

func TestList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	instances := []model.Instance{
		{ContainerID: "c1", Name: "alpha", Status: "Running"},
		{ContainerID: "c2", Name: "beta", Status: "Stopped"},
		{ContainerID: "c3", Name: "gamma", Status: "Running"},
	}
	for _, inst := range instances {
		if err := s.Save(ctx, inst); err != nil {
			t.Fatalf("save %s: %v", inst.Name, err)
		}
	}

	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 instances, got %d", len(got))
	}
}

func TestList_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(got))
	}
}
