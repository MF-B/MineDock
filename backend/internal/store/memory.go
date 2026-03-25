package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"minedock/backend/internal/model"

	_ "modernc.org/sqlite"
)

var ErrNameExists = errors.New("instance name already exists")

// SQLiteStore persists instance state in a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	cleanPath := strings.TrimSpace(dbPath)
	if cleanPath == "" {
		return nil, errors.New("db path is required")
	}

	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// SQLite works best with a single writer connection in this MVP.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.InitSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) InitSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS instances (
	container_id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("init instances table: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Save(ctx context.Context, instance model.Instance) error {
	const upsert = `
INSERT INTO instances(container_id, name, status)
VALUES(?, ?, ?)
ON CONFLICT(container_id)
DO UPDATE SET
	name = excluded.name,
	status = excluded.status;
`

	_, err := s.db.ExecContext(ctx, upsert, instance.ContainerID, instance.Name, instance.Status)
	if err != nil {
		if isUniqueNameErr(err) {
			return ErrNameExists
		}
		return fmt.Errorf("save instance: %w", err)
	}

	return nil
}

func (s *SQLiteStore) Delete(ctx context.Context, containerID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM instances WHERE container_id = ?", containerID); err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Get(ctx context.Context, containerID string) (model.Instance, bool, error) {
	const q = `
SELECT container_id, name, status
FROM instances
WHERE container_id = ?;
`

	var inst model.Instance
	err := s.db.QueryRowContext(ctx, q, containerID).Scan(&inst.ContainerID, &inst.Name, &inst.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Instance{}, false, nil
	}
	if err != nil {
		return model.Instance{}, false, fmt.Errorf("get instance: %w", err)
	}
	return inst, true, nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]model.Instance, error) {
	const q = `
SELECT container_id, name, status
FROM instances
ORDER BY created_at DESC;
`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	defer rows.Close()

	out := make([]model.Instance, 0)
	for rows.Next() {
		var inst model.Instance
		if err := rows.Scan(&inst.ContainerID, &inst.Name, &inst.Status); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		out = append(out, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instances: %w", err)
	}

	return out, nil
}

func isUniqueNameErr(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "UNIQUE constraint failed: instances.name")
}
