package operational

import (
	"context"
	"errors"
	"fmt"
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
	Put(Snapshot) error
	All() []Snapshot
}

type errorAwareStore interface {
	GetWithError(string) (Snapshot, bool, error)
}

func getSnapshot(store Store, name string) (Snapshot, bool, error) {
	if aware, ok := store.(errorAwareStore); ok {
		return aware.GetWithError(name)
	}
	snapshot, found := store.Get(name)
	return snapshot, found, nil
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

func (s *MemoryStore) GetWithError(name string) (Snapshot, bool, error) {
	return s.Get(name)
}

func (s *MemoryStore) Put(snapshot Snapshot) error {
	if snapshot.ProviderName == "" {
		return errors.New("provider name is required")
	}
	s.mu.Lock(); defer s.mu.Unlock()
	s.snapshots[snapshot.ProviderName] = snapshot
	return nil
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
	var balanceProvider provider.BalanceProvider
	if implementation, err := s.Registry.GetCapabilityProvider(name, provider.CapabilityBalance); err == nil {
		var ok bool
		balanceProvider, ok = implementation.(provider.BalanceProvider)
		if !ok { return Snapshot{}, provider.ErrUnsupportedOperation }
	} else {
		// Compatibility path for legacy PPOBProvider registrations that already
		// implement BalanceProvider. New partial capability adapters should use
		// the explicit capability registry boundary above.
		p, getErr := s.Registry.Get(name)
		if getErr != nil { return Snapshot{}, getErr }
		var ok bool
		balanceProvider, ok = p.(provider.BalanceProvider)
		if !ok { return Snapshot{}, provider.ErrUnsupportedOperation }
	}
	now := s.Now()
	previous, _, storeGetErr := getSnapshot(s.Store, name)
	if storeGetErr != nil {
		return Snapshot{}, fmt.Errorf("load provider operational snapshot: %w", storeGetErr)
	}
	balance, err := balanceProvider.GetBalance(ctx)
	if err != nil {
		failures := previous.ConsecutiveFailures + 1
		health := HealthDegraded
		if failures >= s.FailureThreshold { health = HealthUnhealthy }
		snapshot := Snapshot{ProviderName:name, Balance:previous.Balance, Currency:s.Currency, Health:health, LastCheckedAt:now, LastSuccessAt:previous.LastSuccessAt, LastError:err.Error(), ConsecutiveFailures:failures}
		if storeErr := s.Store.Put(snapshot); storeErr != nil {
			return Snapshot{}, errors.Join(err, storeErr)
		}
		return snapshot, err
	}
	snapshot := Snapshot{ProviderName:name, Balance:balance, Currency:s.Currency, Health:HealthHealthy, LastCheckedAt:now, LastSuccessAt:now, ConsecutiveFailures:0}
	if storeErr := s.Store.Put(snapshot); storeErr != nil {
		return Snapshot{}, storeErr
	}
	return snapshot, nil
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
