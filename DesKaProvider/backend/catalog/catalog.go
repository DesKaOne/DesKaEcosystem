package catalog

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var ErrSnapshotOlder = errors.New("catalog snapshot is older than stored snapshot")
var ErrStatusPersistence = errors.New("catalog sync status persistence failed")

type Snapshot struct {
	ProviderName string             `json:"provider_name"`
	Products     []provider.Product `json:"products"`
	SyncedAt     time.Time          `json:"synced_at"`
}

type Store interface {
	Get(string) (Snapshot, bool)
	Put(Snapshot) error
	All() []Snapshot
}

type MemoryStore struct {
	mu        sync.RWMutex
	snapshots map[string]Snapshot
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{snapshots: make(map[string]Snapshot)} }

func (s *MemoryStore) Get(name string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.snapshots[name]
	v.Products = append([]provider.Product(nil), v.Products...)
	return v, ok
}

func (s *MemoryStore) Put(snapshot Snapshot) error {
	if snapshot.ProviderName == "" {
		return errors.New("provider name is required")
	}
	if snapshot.SyncedAt.IsZero() {
		return errors.New("catalog sync time is required")
	}
	snapshot.Products = append([]provider.Product(nil), snapshot.Products...)
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.snapshots[snapshot.ProviderName]; ok && snapshot.SyncedAt.Before(current.SyncedAt) {
		return ErrSnapshotOlder
	}
	s.snapshots[snapshot.ProviderName] = snapshot
	return nil
}

func (s *MemoryStore) All() []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Snapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		snapshot.Products = append([]provider.Product(nil), snapshot.Products...)
		result = append(result, snapshot)
	}
	return result
}

type SyncStatus struct {
	ProviderName        string
	LastAttemptAt       time.Time
	LastSuccessAt       time.Time
	LastError           string
	ConsecutiveFailures int
}

type SyncService struct {
	Registry          *provider.Registry
	Store             Store
	StatusPersistence StatusPersistence
	Now               func() time.Time

	statusMu                    sync.RWMutex
	statuses                    map[string]SyncStatus
	statusPersistenceErr        error
	statusPersistenceFailures   int
}

func NewSyncService(registry *provider.Registry, store Store) (*SyncService, error) {
	if registry == nil {
		return nil, errors.New("provider registry is required")
	}
	if store == nil {
		return nil, errors.New("catalog store is required")
	}
	return &SyncService{
		Registry: registry,
		Store:    store,
		Now:      time.Now,
		statuses: make(map[string]SyncStatus),
	}, nil
}

func NewSyncServiceWithStatusPersistence(registry *provider.Registry, store Store, persistence StatusPersistence) (*SyncService, error) {
	svc, err := NewSyncService(registry, store)
	if err != nil {
		return nil, err
	}
	if persistence == nil {
		return nil, errors.New("catalog sync status persistence is required")
	}
	statuses, err := persistence.Load()
	if err != nil {
		return nil, err
	}
	for _, status := range statuses {
		if status.ProviderName == "" || status.ConsecutiveFailures < 0 {
			return nil, errors.New("invalid catalog sync status")
		}
		svc.statuses[status.ProviderName] = status
	}
	svc.StatusPersistence = persistence
	return svc, nil
}

func (s *SyncService) SyncProvider(ctx context.Context, name string) (Snapshot, error) {
	attemptAt := s.Now()
	s.recordAttempt(name, attemptAt)

	p, err := s.Registry.Get(name)
	if err != nil {
		s.recordFailure(name, attemptAt, err)
		return Snapshot{}, err
	}
	products, err := p.GetProducts(ctx, provider.ProductRequest{})
	if err != nil {
		s.recordFailure(name, attemptAt, err)
		return Snapshot{}, err
	}

	snapshot := Snapshot{
		ProviderName: name,
		Products:     append([]provider.Product(nil), products...),
		SyncedAt:     s.Now(),
	}
	if err := s.Store.Put(snapshot); err != nil {
		s.recordFailure(name, attemptAt, err)
		return Snapshot{}, err
	}
	s.recordSuccess(name, attemptAt, snapshot.SyncedAt)
	return snapshot, nil
}

func (s *SyncService) recordAttempt(name string, at time.Time) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	status := s.statuses[name]
	status.ProviderName = name
	status.LastAttemptAt = at
	s.statuses[name] = status
	s.persistStatusesLocked()
}

func (s *SyncService) recordFailure(name string, attemptAt time.Time, err error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	status := s.statuses[name]
	status.ProviderName = name
	status.LastAttemptAt = attemptAt
	status.LastError = err.Error()
	status.ConsecutiveFailures++
	s.statuses[name] = status
	s.persistStatusesLocked()
}

func (s *SyncService) recordSuccess(name string, attemptAt, successAt time.Time) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	status := s.statuses[name]
	status.ProviderName = name
	status.LastAttemptAt = attemptAt
	status.LastSuccessAt = successAt
	status.LastError = ""
	status.ConsecutiveFailures = 0
	s.statuses[name] = status
	s.persistStatusesLocked()
}

func (s *SyncService) persistStatusesLocked() {
	if s.StatusPersistence == nil {
		return
	}
	statuses := make([]SyncStatus, 0, len(s.statuses))
	for _, status := range s.statuses {
		statuses = append(statuses, status)
	}
	if err := s.StatusPersistence.Save(statuses); err != nil {
		s.statusPersistenceErr = fmt.Errorf("%w: %v", ErrStatusPersistence, err)
		s.statusPersistenceFailures++
		return
	}
	s.statusPersistenceErr = nil
	s.statusPersistenceFailures = 0
}

func (s *SyncService) StatusPersistenceError() error {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.statusPersistenceErr
}

func (s *SyncService) StatusPersistenceFailures() int {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.statusPersistenceFailures
}

func (s *SyncService) Status(name string) (SyncStatus, bool) {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	status, ok := s.statuses[name]
	return status, ok
}

func (s *SyncService) Statuses() []SyncStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	result := make([]SyncStatus, 0, len(s.statuses))
	for _, status := range s.statuses {
		result = append(result, status)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ProviderName < result[j].ProviderName
	})
	return result
}

func (s *SyncService) SyncAll(ctx context.Context) map[string]error {
	errorsByProvider := make(map[string]error)
	for _, name := range s.Registry.Names() {
		if err := ctx.Err(); err != nil {
			break
		}
		if _, err := s.SyncProvider(ctx, name); err != nil {
			errorsByProvider[name] = err
		}
		if err := ctx.Err(); err != nil {
			break
		}
	}
	return errorsByProvider
}
