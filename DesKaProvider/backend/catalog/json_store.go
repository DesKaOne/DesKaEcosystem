package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type JSONFileStore struct {
	mu sync.RWMutex
	path string
	data map[string]Snapshot
	ambiguous bool
	syncDirectory func(string) error
}

type fileData struct { Snapshots map[string]Snapshot `json:"snapshots"` }

func NewJSONFileStore(path string) (*JSONFileStore, error) {
	if path == "" { return nil, errors.New("catalog store path is required") }
	s := &JSONFileStore{path:path, data:make(map[string]Snapshot), syncDirectory:syncDirectory}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) { return s, nil }
	if err != nil { return nil, fmt.Errorf("read catalog store: %w", err) }
	if len(raw) == 0 { return s, nil }
	var payload fileData
	if err := json.Unmarshal(raw, &payload); err != nil { return nil, fmt.Errorf("decode catalog store: %w", err) }
	if payload.Snapshots != nil {
		for name, snapshot := range payload.Snapshots {
			if name == "" || snapshot.ProviderName == "" || snapshot.ProviderName != name || snapshot.SyncedAt.IsZero() {
				return nil, fmt.Errorf("invalid catalog snapshot %q", name)
			}
			snapshot.Products = append([]provider.Product(nil), snapshot.Products...)
			s.data[name] = snapshot
		}
	}
	return s, nil
}

func (s *JSONFileStore) Get(name string) (Snapshot, bool) {
	s.mu.RLock(); defer s.mu.RUnlock()
	if s.ambiguous { return Snapshot{}, false }
	v, ok := s.data[name]
	v.Products = append([]provider.Product(nil), v.Products...)
	return v, ok
}

func (s *JSONFileStore) Put(snapshot Snapshot) error {
	if snapshot.ProviderName == "" { return errors.New("provider name is required") }
	if snapshot.SyncedAt.IsZero() { return errors.New("catalog sync time is required") }
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.data[snapshot.ProviderName]; ok && snapshot.SyncedAt.Before(current.SyncedAt) {
		return ErrSnapshotOlder
	}
	next := make(map[string]Snapshot, len(s.data)+1)
	for name, current := range s.data {
		next[name] = Snapshot{ProviderName: current.ProviderName, Products: append([]provider.Product(nil), current.Products...), SyncedAt: current.SyncedAt}
	}
	next[snapshot.ProviderName] = Snapshot{ProviderName:snapshot.ProviderName, Products:append([]provider.Product(nil), snapshot.Products...), SyncedAt:snapshot.SyncedAt}
	if err := s.persistLocked(next); err != nil {
		if errors.Is(err, ErrCatalogPersistenceAmbiguous) { s.ambiguous = true }
		return err
	}
	s.data = next
	s.ambiguous = false
	return nil
}

func (s *JSONFileStore) All() []Snapshot {
	s.mu.RLock(); defer s.mu.RUnlock()
	if s.ambiguous { return nil }
	names := make([]string,0,len(s.data))
	for name := range s.data { names = append(names,name) }
	sort.Strings(names)
	result := make([]Snapshot,0,len(s.data))
	for _, name := range names { v:=s.data[name]; v.Products=append([]provider.Product(nil),v.Products...); result=append(result,v) }
	return result
}

func (s *JSONFileStore) persistLocked(data map[string]Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil { return err }
	payload, err := json.MarshalIndent(fileData{Snapshots:data},"","  ")
	if err != nil { return err }
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".catalog-*.tmp")
	if err != nil { return err }
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil { tmp.Close(); return err }
	if _, err := tmp.Write(payload); err != nil { tmp.Close(); return err }
	if err := tmp.Sync(); err != nil { tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(tmpName, s.path); err != nil { return err }
	if err := s.syncDirectory(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("%w: %v", ErrCatalogPersistenceAmbiguous, err)
	}
	return nil
}

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil { return err }
	defer f.Close()
	return f.Sync()
}
