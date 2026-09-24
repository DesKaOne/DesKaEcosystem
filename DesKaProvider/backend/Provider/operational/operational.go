package operational

import (
	"context"
	"errors"
	"sync"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

type Health string

const (
	HealthUnknown Health = "unknown"
	HealthHealthy Health = "healthy"
	HealthDegraded Health = "degraded"
	HealthUnhealthy Health = "unhealthy"
)

type Snapshot struct {
	ProviderName string
	Balance int64
	Currency string
	Health Health
	LastCheckedAt time.Time
	LastSuccessAt time.Time
	LastError string
	ConsecutiveFailures int
}

type Store interface {
	Get(string) (Snapshot, bool)
	Put(Snapshot)
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
	return v, ok
}

func (s *MemoryStore) Put(snapshot Snapshot) {
	s.mu.Lock(); defer s.mu.Unlock()
	s.snapshots[snapshot.ProviderName] = snapshot
}

func (s *MemoryStore) All() []Snapshot {
	s.mu.RLock(); defer s.mu.RUnlock()
	result := make([]Snapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots { result = append(result, snapshot) }
	return result
}

type SyncService struct {
	Registry *provider.Registry
	Store Store
	Currency string
	FailureThreshold int
	Now func() time.Time
}

func NewSyncService(registry *provider.Registry, store Store, currency string, failureThreshold int) (*SyncService, error) {
	if registry == nil { return nil, errors.New("provider registry is required") }
	if store == nil { return nil, errors.New("operational store is required") }
	if failureThreshold < 1 { return nil, errors.New("failure threshold must be at least 1") }
	if currency == "" { currency = "IDR" }
	return &SyncService{Registry: registry, Store: store, Currency: currency, FailureThreshold: failureThreshold, Now: time.Now}, nil
}

func (s *SyncService) SyncProvider(ctx context.Context, name string) (Snapshot, error) {
	p, err := s.Registry.Get(name)
	if err != nil { return Snapshot{}, err }
	balanceProvider, ok := p.(provider.BalanceProvider)
	if !ok { return Snapshot{}, provider.ErrUnsupportedOperation }
	now := s.Now()
	previous, _ := s.Store.Get(name)
	balance, err := balanceProvider.GetBalance(ctx)
	if err != nil {
		failures := previous.ConsecutiveFailures + 1
		health := HealthDegraded
		if failures >= s.FailureThreshold { health = HealthUnhealthy }
		snapshot := Snapshot{ProviderName:name, Balance:previous.Balance, Currency:s.Currency, Health:health, LastCheckedAt:now, LastSuccessAt:previous.LastSuccessAt, LastError:err.Error(), ConsecutiveFailures:failures}
		s.Store.Put(snapshot)
		return snapshot, err
	}
	snapshot := Snapshot{ProviderName:name, Balance:balance, Currency:s.Currency, Health:HealthHealthy, LastCheckedAt:now, LastSuccessAt:now, ConsecutiveFailures:0}
	s.Store.Put(snapshot)
	return snapshot, nil
}

func (s *SyncService) SyncAll(ctx context.Context) map[string]error {
	errorsByProvider := make(map[string]error)
	for _, name := range s.Registry.Names() {
		if _, err := s.SyncProvider(ctx, name); err != nil { errorsByProvider[name] = err }
	}
	return errorsByProvider
}

// Run starts the periodic synchronization loop. The first synchronization
// happens immediately, followed by the configured interval.
func (s *SyncService) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 { return errors.New("sync interval must be greater than zero") }
	_ = s.SyncAll(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_ = s.SyncAll(ctx)
		}
	}
}
