package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"minedock/backend/internal/model"
)

const instanceConfigFilename = "minedock.instance.json"

// InstanceConfigFileStore persists desired instance configuration files.
type InstanceConfigFileStore struct {
	dataDir string

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewInstanceConfigFileStore creates an instance config file store.
func NewInstanceConfigFileStore(dataDir string) *InstanceConfigFileStore {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	return &InstanceConfigFileStore{
		dataDir: dataDir,
		locks:   map[string]*sync.Mutex{},
	}
}

// ConfigPath returns the desired config file path for an instance name.
func (s *InstanceConfigFileStore) ConfigPath(instanceName string) (string, error) {
	root, err := safeInstanceDataDir(s.dataDir, instanceName)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, instanceConfigFilename), nil
}

// Load loads a desired instance configuration from path.
func (s *InstanceConfigFileStore) Load(ctx context.Context, configPath string) (*model.StoredInstanceConfig, error) {
	cleanPath := strings.TrimSpace(configPath)
	if cleanPath == "" {
		return nil, model.ErrFileNotFound
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.ErrFileNotFound
		}
		return nil, fmt.Errorf("read instance config: %w", err)
	}

	var cfg model.StoredInstanceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode instance config: %w", err)
	}
	return &cfg, nil
}

// Save writes a desired instance configuration atomically to path.
func (s *InstanceConfigFileStore) Save(ctx context.Context, configPath string, cfg *model.StoredInstanceConfig) error {
	cleanPath := strings.TrimSpace(configPath)
	if cleanPath == "" {
		return model.ErrPathInvalid
	}
	if cfg == nil {
		return model.ErrInvalidParams
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("resolve instance config path: %w", err)
	}

	lock := s.pathLock(absPath)
	lock.Lock()
	defer lock.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("create instance config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode instance config: %w", err)
	}
	data = append(data, '\n')

	tmpPath := absPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open temp instance config: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp instance config: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync temp instance config: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp instance config: %w", err)
	}

	if err := os.Rename(tmpPath, absPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace instance config: %w", err)
	}

	return nil
}

func (s *InstanceConfigFileStore) pathLock(absPath string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock, ok := s.locks[absPath]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[absPath] = lock
	}
	return lock
}
