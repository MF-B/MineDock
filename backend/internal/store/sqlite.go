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

	_ "modernc.org/sqlite" // 注册 SQLite 驱动
)

// SQLiteStore 在本地 SQLite 数据库中持久化实例元数据。
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 打开 dbPath 指向的 SQLite 并初始化所需表结构。
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

	// 说明：当前 MVP 阶段使用单写连接更贴合 SQLite 的并发模型。
	// TODO: 当写入吞吐成为瓶颈时，重新评估连接池策略。
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

// Close 释放底层 SQLite 数据库连接。
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// InitSchema 在表不存在时创建存储表结构。
func (s *SQLiteStore) InitSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS instances (
	container_id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	game_id TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("init instances table: %w", err)
	}
	if err := s.ensureGameIDColumn(ctx); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) ensureGameIDColumn(ctx context.Context) error {
	const addGameIDColumn = `
ALTER TABLE instances
ADD COLUMN game_id TEXT NOT NULL DEFAULT '';
`

	if _, err := s.db.ExecContext(ctx, addGameIDColumn); err != nil {
		if isDuplicateColumnErr(err) {
			return nil
		}
		return fmt.Errorf("ensure instances.game_id column: %w", err)
	}

	return nil
}

// Save 按容器 ID 执行实例记录的 upsert。
func (s *SQLiteStore) Save(ctx context.Context, instance model.Instance) error {
	const upsert = `
INSERT INTO instances(container_id, name, game_id)
VALUES(?, ?, ?)
ON CONFLICT(container_id)
DO UPDATE SET
	name = excluded.name,
	game_id = excluded.game_id;
`

	_, err := s.db.ExecContext(ctx, upsert, instance.ContainerID, instance.Name, instance.GameID)
	if err != nil {
		if isUniqueNameErr(err) {
			return model.ErrNameExists
		}
		return fmt.Errorf("save instance: %w", err)
	}

	return nil
}

// Delete 按容器 ID 删除一条实例记录。
func (s *SQLiteStore) Delete(ctx context.Context, containerID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM instances WHERE container_id = ?", containerID); err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	return nil
}

// Get 按容器 ID 获取一条实例记录。
func (s *SQLiteStore) Get(ctx context.Context, containerID string) (model.Instance, bool, error) {
	const q = `
SELECT container_id, name, game_id
FROM instances
WHERE container_id = ?;
`

	var inst model.Instance
	err := s.db.QueryRowContext(ctx, q, containerID).Scan(&inst.ContainerID, &inst.Name, &inst.GameID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Instance{}, false, nil
	}
	if err != nil {
		return model.Instance{}, false, fmt.Errorf("get instance: %w", err)
	}
	return inst, true, nil
}

// List 返回所有实例记录，并按创建时间倒序排列。
func (s *SQLiteStore) List(ctx context.Context) ([]model.Instance, error) {
	const q = `
SELECT container_id, name, game_id
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
		if err := rows.Scan(&inst.ContainerID, &inst.Name, &inst.GameID); err != nil {
			return nil, fmt.Errorf("scan instance: %w", err)
		}
		out = append(out, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instances: %w", err)
	}

	return out, nil
}

// isUniqueNameErr 判断是否为 instances.name 的唯一约束冲突。
// 说明：当前实现依赖 SQLite 错误文本匹配。
// TODO: 用 SQLite 错误码替代文本匹配判断。
func isUniqueNameErr(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "UNIQUE constraint failed: instances.name")
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate column name") && strings.Contains(errStr, "game_id")
}
