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
}

type fileData struct { Snapshots map[string]Snapshot `json:"snapshots"` }

func NewJSONFileStore(path string) (*JSONFileStore, error) {
	if path == "" { return nil, errors.New("catalog store path is required") }
	s := &JSONFileStore{path:path, data:make(map[string]Snapshot)}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || len(raw) == 0 { return s, nil }
	if err != nil { return nil, err }
	var payload fileData
	if err := json.Unmarshal(raw, &payload); err != nil { return nil, fmt.Errorf("decode catalog store: %w", err) }
	if payload.Snapshots != nil { s.data = payload.Snapshots }
	return s, nil
}

func (s *JSONFileStore) Get(name string) (Snapshot, bool) {
	s.mu.RLock(); defer s.mu.RUnlock()
	v, ok := s.data[name]
	v.Products = append([]provider.Product(nil), v.Products...)
	return v, ok
}

func (s *JSONFileStore) Put(snapshot Snapshot) error {
	if snapshot.ProviderName == "" { return errors.New("provider name is required") }
	if snapshot.SyncedAt.IsZero() { return errors.New("catalog sync time is required") }
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[snapshot.ProviderName] = Snapshot{ProviderName:snapshot.ProviderName, Products:append([]provider.Product(nil), snapshot.Products...), SyncedAt:snapshot.SyncedAt}
	return s.persistLocked()
}

func (s *JSONFileStore) All() []Snapshot {
	s.mu.RLock(); defer s.mu.RUnlock()
	names := make([]string,0,len(s.data))
	for name := range s.data { names = append(names,name) }
	sort.Strings(names)
	result := make([]Snapshot,0,len(names))
	for _, name := range names { v:=s.data[name]; v.Products=append([]provider.Product(nil),v.Products...); result=append(result,v) }
	return result
}

func (s *JSONFileStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil { return err }
	payload, err := json.MarshalIndent(fileData{Snapshots:s.data},"","  ")
	if err != nil { return err }
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".catalog-*.tmp")
	if err != nil { return err }
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil { tmp.Close(); return err }
	if _, err := tmp.Write(payload); err != nil { tmp.Close(); return err }
	if err := tmp.Sync(); err != nil { tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	return os.Rename(tmpName,s.path)
}
