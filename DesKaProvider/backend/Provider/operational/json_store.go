package operational

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type JSONFileStore struct {
	mu sync.RWMutex
	path string
	snapshots map[string]Snapshot
}

type jsonFileState struct {
	Snapshots map[string]Snapshot `json:"snapshots"`
}

func NewJSONFileStore(path string) (*JSONFileStore, error) {
	if path == "" {
		return nil, errors.New("operational store path is required")
	}
	store := &JSONFileStore{path: path, snapshots: make(map[string]Snapshot)}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read operational store: %w", err)
	}
	if len(data) == 0 {
		return store, nil
	}
	var state jsonFileState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode operational store: %w", err)
	}
	if state.Snapshots != nil {
		store.snapshots = state.Snapshots
	}
	return store, nil
}

func (s *JSONFileStore) Get(name string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.snapshots[name]
	return v, ok
}

func (s *JSONFileStore) All() []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.snapshots))
	for name := range s.snapshots {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]Snapshot, 0, len(names))
	for _, name := range names {
		result = append(result, s.snapshots[name])
	}
	return result
}

func (s *JSONFileStore) Put(snapshot Snapshot) error {
	if snapshot.ProviderName == "" {
		return errors.New("provider name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[snapshot.ProviderName] = snapshot
	return s.persistLocked()
}

func (s *JSONFileStore) persistLocked() error {
	state := jsonFileState{Snapshots: s.snapshots}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode operational store: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create operational store directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".operational-*.tmp")
	if err != nil {
		return fmt.Errorf("create operational store temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	defer cleanup()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("secure operational store temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write operational store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync operational store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close operational store: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace operational store: %w", err)
	}
	return nil
}
