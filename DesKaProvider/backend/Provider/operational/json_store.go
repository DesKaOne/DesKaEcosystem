package operational

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type operationalStorePersistStage string

const (
	operationalStoreBeforeReplace operationalStorePersistStage = "before-replace"
	operationalStoreAfterReplace  operationalStorePersistStage = "after-replace"
)

type JSONFileStore struct {
	mu sync.RWMutex
	path string
	snapshots map[string]Snapshot
	persistHook func(operationalStorePersistStage) error
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
		for name, snapshot := range state.Snapshots {
			if err := ValidateSnapshot(snapshot); err != nil {
				return nil, fmt.Errorf("validate operational snapshot %q: %w", name, err)
			}
		}
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

func (s *JSONFileStore) GetWithError(name string) (Snapshot, bool, error) {
	snapshot, found := s.Get(name)
	return snapshot, found, nil
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

func moreRestrictiveHealth(current, requested Health) Health {
	if current == HealthUnhealthy || requested == HealthUnhealthy {
		return HealthUnhealthy
	}
	if current == HealthDegraded || requested == HealthDegraded {
		return HealthDegraded
	}
	if current == HealthUnknown || requested == HealthUnknown {
		return HealthUnknown
	}
	return HealthHealthy
}

func safeSnapshotAfterAmbiguousPersistence(current, requested Snapshot) Snapshot {
	if current.ProviderName == "" {
		return Snapshot{
			ProviderName: requested.ProviderName,
			Balance: 0,
			Currency: requested.Currency,
			Health: HealthUnknown,
			LastCheckedAt: requested.LastCheckedAt,
			ConsecutiveFailures: 0,
		}
	}
	result := requested
	if current.ProviderName != "" {
		result.ProviderName = current.ProviderName
	}
	if current.Currency != "" {
		result.Currency = current.Currency
	}
	if requested.Balance > current.Balance {
		result.Balance = current.Balance
	}
	result.Health = moreRestrictiveHealth(current.Health, requested.Health)
	if current.LastCheckedAt.IsZero() || requested.LastCheckedAt.IsZero() {
		result.LastCheckedAt = time.Time{}
	} else if current.LastCheckedAt.Before(requested.LastCheckedAt) {
		result.LastCheckedAt = current.LastCheckedAt
	}
	if current.LastSuccessAt.IsZero() || requested.LastSuccessAt.IsZero() {
		result.LastSuccessAt = time.Time{}
	} else if current.LastSuccessAt.Before(requested.LastSuccessAt) {
		result.LastSuccessAt = current.LastSuccessAt
	}
	if current.ConsecutiveFailures > result.ConsecutiveFailures {
		result.ConsecutiveFailures = current.ConsecutiveFailures
	}
	if current.LastError != "" {
		result.LastError = current.LastError
	}
	return result
}

func (s *JSONFileStore) Put(snapshot Snapshot) error {
	if err := ValidateSnapshot(snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make(map[string]Snapshot, len(s.snapshots)+1)
	for name, existing := range s.snapshots {
		next[name] = existing
	}
	next[snapshot.ProviderName] = snapshot

	if err := s.persist(next); err != nil {
		if errors.Is(err, ErrOperationalPersistenceAmbiguous) {
			s.snapshots[snapshot.ProviderName] = safeSnapshotAfterAmbiguousPersistence(s.snapshots[snapshot.ProviderName], snapshot)
		}
		return err
	}
	s.snapshots = next
	return nil
}

func syncJSONStoreDirectory(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func (s *JSONFileStore) persist(snapshots map[string]Snapshot) error {
	state := jsonFileState{Snapshots: snapshots}
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
	if err := syncJSONStoreDirectory(dir); err != nil {
		return fmt.Errorf("sync operational store directory: %w", err)
	}
	return nil
}
