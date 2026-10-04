package runtime

import (
	"sync"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

// recoveryFenceState records which providers have produced a successful
// durable observation in the current runtime generation. Persisted observations
// from an earlier generation remain available to synchronization itself, but
// are hidden from routing until this generation successfully refreshes them.
type recoveryFenceState struct {
	mu    sync.RWMutex
	ready map[string]bool
}

func newRecoveryFenceState() *recoveryFenceState {
	return &recoveryFenceState{ready: make(map[string]bool)}
}

func (s *recoveryFenceState) markReady(name string) {
	if s == nil || name == "" {
		return
	}
	s.mu.Lock()
	s.ready[name] = true
	s.mu.Unlock()
}

func (s *recoveryFenceState) isReady(name string) bool {
	if s == nil || name == "" {
		return false
	}
	s.mu.RLock()
	ready := s.ready[name]
	s.mu.RUnlock()
	return ready
}

// operationalSyncStore preserves access to the previous durable observation
// for synchronization, while marking a provider ready only after Put returns
// success. Ambiguous persistence never opens the routing gate.
type operationalSyncStore struct {
	underlying operational.Store
	fence      *recoveryFenceState
}

func newOperationalSyncStore(store operational.Store, fence *recoveryFenceState) operational.Store {
	return &operationalSyncStore{underlying: store, fence: fence}
}

func (s *operationalSyncStore) Get(name string) (operational.Snapshot, bool) {
	return s.underlying.Get(name)
}

func (s *operationalSyncStore) GetWithError(name string) (operational.Snapshot, bool, error) {
	if aware, ok := s.underlying.(interface {
		GetWithError(string) (operational.Snapshot, bool, error)
	}); ok {
		return aware.GetWithError(name)
	}
	snapshot, found := s.underlying.Get(name)
	return snapshot, found, nil
}

func (s *operationalSyncStore) Put(snapshot operational.Snapshot) error {
	if err := s.underlying.Put(snapshot); err != nil {
		return err
	}
	s.fence.markReady(snapshot.ProviderName)
	return nil
}

func (s *operationalSyncStore) All() []operational.Snapshot {
	return s.underlying.All()
}

// operationalRoutingStore is read-only in practice: routing sees only a
// successful current-generation observation. It still implements Store so the
// existing routing constructor can remain unchanged.
type operationalRoutingStore struct {
	underlying operational.Store
	fence      *recoveryFenceState
}

func newOperationalRoutingStore(store operational.Store, fence *recoveryFenceState) operational.Store {
	return &operationalRoutingStore{underlying: store, fence: fence}
}

func (s *operationalRoutingStore) Get(name string) (operational.Snapshot, bool) {
	if !s.fence.isReady(name) {
		return operational.Snapshot{}, false
	}
	return s.underlying.Get(name)
}

func (s *operationalRoutingStore) GetWithError(name string) (operational.Snapshot, bool, error) {
	if !s.fence.isReady(name) {
		return operational.Snapshot{}, false, nil
	}
	if aware, ok := s.underlying.(interface {
		GetWithError(string) (operational.Snapshot, bool, error)
	}); ok {
		return aware.GetWithError(name)
	}
	snapshot, found := s.underlying.Get(name)
	return snapshot, found, nil
}

func (s *operationalRoutingStore) Put(snapshot operational.Snapshot) error {
	return s.underlying.Put(snapshot)
}

func (s *operationalRoutingStore) All() []operational.Snapshot {
	if s == nil || s.underlying == nil {
		return nil
	}
	all := s.underlying.All()
	result := make([]operational.Snapshot, 0, len(all))
	for _, snapshot := range all {
		if s.fence.isReady(snapshot.ProviderName) {
			result = append(result, snapshot)
		}
	}
	return result
}

// catalogSyncStore keeps durable catalog state readable by the synchronizer,
// while opening the current-generation routing gate only after a successful
// durable Put.
type catalogSyncStore struct {
	underlying catalog.Store
	fence      *recoveryFenceState
}

func newCatalogSyncStore(store catalog.Store, fence *recoveryFenceState) catalog.Store {
	return &catalogSyncStore{underlying: store, fence: fence}
}

func (s *catalogSyncStore) Get(name string) (catalog.Snapshot, bool) {
	return s.underlying.Get(name)
}

func (s *catalogSyncStore) Put(snapshot catalog.Snapshot) error {
	if err := s.underlying.Put(snapshot); err != nil {
		return err
	}
	s.fence.markReady(snapshot.ProviderName)
	return nil
}

func (s *catalogSyncStore) All() []catalog.Snapshot {
	return s.underlying.All()
}

// catalogRoutingStore hides persisted catalog data until the current runtime
// generation has durably refreshed that provider's catalog.
type catalogRoutingStore struct {
	underlying catalog.Store
	fence      *recoveryFenceState
}

func newCatalogRoutingStore(store catalog.Store, fence *recoveryFenceState) catalog.Store {
	return &catalogRoutingStore{underlying: store, fence: fence}
}

func (s *catalogRoutingStore) Get(name string) (catalog.Snapshot, bool) {
	if !s.fence.isReady(name) {
		return catalog.Snapshot{}, false
	}
	return s.underlying.Get(name)
}

func (s *catalogRoutingStore) Put(snapshot catalog.Snapshot) error {
	return s.underlying.Put(snapshot)
}

func (s *catalogRoutingStore) All() []catalog.Snapshot {
	if s == nil || s.underlying == nil {
		return nil
	}
	all := s.underlying.All()
	result := make([]catalog.Snapshot, 0, len(all))
	for _, snapshot := range all {
		if s.fence.isReady(snapshot.ProviderName) {
			result = append(result, snapshot)
		}
	}
	return result
}
