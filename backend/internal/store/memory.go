package store

import (
	"errors"
	"sync"

	"minedock/backend/internal/model"
)

var ErrNameExists = errors.New("instance name already exists")

// MemoryStore stores instance state in memory only.
type MemoryStore struct {
	mu     sync.RWMutex
	byID   map[string]model.Instance
	byName map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byID:   make(map[string]model.Instance),
		byName: make(map[string]string),
	}
}

func (s *MemoryStore) Save(instance model.Instance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, ok := s.byName[instance.Name]; ok && existingID != instance.ContainerID {
		return ErrNameExists
	}

	s.byID[instance.ContainerID] = instance
	s.byName[instance.Name] = instance.ContainerID
	return nil
}

func (s *MemoryStore) Delete(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if inst, ok := s.byID[containerID]; ok {
		delete(s.byName, inst.Name)
	}
	delete(s.byID, containerID)
}

func (s *MemoryStore) Get(containerID string) (model.Instance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inst, ok := s.byID[containerID]
	return inst, ok
}

func (s *MemoryStore) List() []model.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Instance, 0, len(s.byID))
	for _, inst := range s.byID {
		out = append(out, inst)
	}
	return out
}
