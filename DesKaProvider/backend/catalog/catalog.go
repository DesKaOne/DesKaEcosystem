package catalog

import (
	"context"
	"errors"
	"sync"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type Snapshot struct {
	ProviderName string          `json:"provider_name"`
	Products     []provider.Product `json:"products"`
	SyncedAt     time.Time       `json:"synced_at"`
}

type Store interface {
	Get(string) (Snapshot, bool)
	Put(Snapshot) error
	All() []Snapshot
}

type MemoryStore struct {
	mu sync.RWMutex
	snapshots map[string]Snapshot
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{snapshots: make(map[string]Snapshot)} }

func (s *MemoryStore) Get(name string) (Snapshot, bool) {
	s.mu.RLock(); defer s.mu.RUnlock()
	v, ok := s.snapshots[name]
	v.Products = append([]provider.Product(nil), v.Products...)
	return v, ok
}

func (s *MemoryStore) Put(snapshot Snapshot) error {
	if snapshot.ProviderName == "" { return errors.New("provider name is required") }
	if snapshot.SyncedAt.IsZero() { return errors.New("catalog sync time is required") }
	snapshot.Products = append([]provider.Product(nil), snapshot.Products...)
	s.mu.Lock(); defer s.mu.Unlock()
	s.snapshots[snapshot.ProviderName] = snapshot
	return nil
}

func (s *MemoryStore) All() []Snapshot {
	s.mu.RLock(); defer s.mu.RUnlock()
	result := make([]Snapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		snapshot.Products = append([]provider.Product(nil), snapshot.Products...)
		result = append(result, snapshot)
	}
	return result
}

type SyncService struct {
	Registry *provider.Registry
	Store Store
	Now func() time.Time
}

func NewSyncService(registry *provider.Registry, store Store) (*SyncService, error) {
	if registry == nil { return nil, errors.New("provider registry is required") }
	if store == nil { return nil, errors.New("catalog store is required") }
	return &SyncService{Registry: registry, Store: store, Now: time.Now}, nil
}

func (s *SyncService) SyncProvider(ctx context.Context, name string) (Snapshot, error) {
	p, err := s.Registry.Get(name)
	if err != nil { return Snapshot{}, err }
	products, err := p.GetProducts(ctx, provider.ProductRequest{})
	if err != nil { return Snapshot{}, err }
	snapshot := Snapshot{ProviderName: name, Products: append([]provider.Product(nil), products...), SyncedAt: s.Now()}
	if err := s.Store.Put(snapshot); err != nil { return Snapshot{}, err }
	return snapshot, nil
}

func (s *SyncService) SyncAll(ctx context.Context) map[string]error {
	errorsByProvider := make(map[string]error)
	for _, name := range s.Registry.Names() {
		if _, err := s.SyncProvider(ctx, name); err != nil { errorsByProvider[name] = err }
	}
	return errorsByProvider
}
