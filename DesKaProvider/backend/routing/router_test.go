package routing

import (
	"context"
	"errors"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
)

func TestRouterSelectsHealthyProviderWithSufficientBalanceAndPriority(t *testing.T) {
	registry := provider.NewRegistry()
	for _, name := range []string{"primary", "secondary"} {
		if err := registry.Register(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		})); err != nil {
			t.Fatal(err)
		}
	}

	store := operational.NewMemoryStore()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	for _, snapshot := range []operational.Snapshot{
		{ProviderName: "primary", Balance: 100000, Health: operational.HealthHealthy, LastCheckedAt: now},
		{ProviderName: "secondary", Balance: 500000, Health: operational.HealthHealthy, LastCheckedAt: now},
	} {
		if err := store.Put(snapshot); err != nil {
			t.Fatal(err)
		}
	}

	router, err := New(registry, store, map[string]int{"primary": 10, "secondary": 20})
	if err != nil {
		t.Fatal(err)
	}
	got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if got != "primary" {
		t.Fatalf("expected primary, got %q", got)
	}
}

func TestRouterRejectsUnavailableOperationalCandidates(t *testing.T) {
	registry := provider.NewRegistry()
	for _, name := range []string{"unhealthy", "low-balance"} {
		if err := registry.Register(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		})); err != nil {
			t.Fatal(err)
		}
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "unhealthy", Balance: 100000, Health: operational.HealthUnhealthy}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(operational.Snapshot{ProviderName: "low-balance", Balance: 1000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}

	router, err := New(registry, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expected no-provider error, got %v", err)
	}
}

func TestRouterRequiresProductAvailability(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{
		Products: []provider.Product{{Code: "other", Name: "Other"}},
	})); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}

	router, err := New(registry, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expected no-provider error, got %v", err)
	}
}

func TestRouterTieBreaksByProviderName(t *testing.T) {
	registry := provider.NewRegistry()
	for _, name := range []string{"alpha", "beta"} {
		if err := registry.Register(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		})); err != nil {
			t.Fatal(err)
		}
	}
	store := operational.NewMemoryStore()
	for _, name := range []string{"alpha", "beta"} {
		if err := store.Put(operational.Snapshot{ProviderName: name, Balance: 100000, Health: operational.HealthHealthy}); err != nil {
			t.Fatal(err)
		}
	}
	router, err := New(registry, store, map[string]int{"alpha": 10, "beta": 10})
	if err != nil {
		t.Fatal(err)
	}
	got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if got != "alpha" {
		t.Fatalf("expected deterministic alpha tie-break, got %q", got)
	}
}

func TestRouterValidatesRequest(t *testing.T) {
	registry := provider.NewRegistry()
	store := operational.NewMemoryStore()
	router, err := New(registry, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.Select(context.Background(), Request{ProductCode: "xld10"}); !errors.Is(err, ErrInvalidRouteRequest) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}

func TestRouterUsesCatalogSnapshotWithoutProviderProductLookup(t *testing.T) {
	registry := provider.NewRegistry()
	mock := mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}
	catalogStore := catalog.NewMemoryStore()
	if err := catalogStore.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		SyncedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	router, err := NewWithCatalog(registry, store, nil, catalogStore)
	if err != nil {
		t.Fatal(err)
	}
	got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if got != "mock" {
		t.Fatalf("expected catalog-backed mock selection, got %q", got)
	}
}

func TestRouterRejectsStaleCatalogSnapshot(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}
	catalogStore := catalog.NewMemoryStore()
	if err := catalogStore.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		SyncedAt: time.Now().Add(-2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	router, err := NewWithCatalogMaxAge(registry, store, nil, catalogStore, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expected stale catalog to remove provider from candidates, got %v", err)
	}
}

func TestNewWithCatalogMaxAgeRejectsInvalidMaxAge(t *testing.T) {
	registry := provider.NewRegistry()
	store := operational.NewMemoryStore()
	catalogStore := catalog.NewMemoryStore()
	if _, err := NewWithCatalogMaxAge(registry, store, nil, catalogStore, 0); err == nil {
		t.Fatal("expected invalid catalog max age error")
	}
}


func TestRouterRequiresEnabledPPOBCapabilityWhenProviderStateIsConfigured(t *testing.T) {
	registry := provider.NewRegistry()
	for _, name := range []string{"disabled", "wrong-capability", "healthy"} {
		if err := registry.Register(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		})); err != nil {
			t.Fatal(err)
		}
	}
	store := operational.NewMemoryStore()
	for _, name := range []string{"disabled", "wrong-capability", "healthy"} {
		if err := store.Put(operational.Snapshot{
			ProviderName: name,
			Balance:      100000,
			Health:       operational.HealthHealthy,
		}); err != nil {
			t.Fatal(err)
		}
	}
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{
		ProviderName: "disabled",
		Lifecycle:    operational.LifecycleDisabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}
	if err := states.Put(operational.ProviderState{
		ProviderName: "wrong-capability",
		Lifecycle:    operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityBalance},
	}); err != nil {
		t.Fatal(err)
	}
	if err := states.Put(operational.ProviderState{
		ProviderName: "healthy",
		Lifecycle:    operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}

	router, err := NewWithState(registry, store, nil, states)
	if err != nil {
		t.Fatal(err)
	}
	got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if got != "healthy" {
		t.Fatalf("expected state-eligible provider, got %q", got)
	}
}

func TestRouterStateGateRejectsAllWhenNoProviderIsEnabledForCapability(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{
		Products: []provider.Product{{Code: "xld10", Name: "Test"}},
	})); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance:      100000,
		Health:       operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle:    operational.LifecycleDisabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}
	router, err := NewWithState(registry, store, nil, states)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000}); !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expected no-provider error, got %v", err)
	}
}

func TestRouterRejectsUnhealthyEnabledProvider(t *testing.T) {
 registry := provider.NewRegistry()
 if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil { t.Fatal(err) }
 store := operational.NewMemoryStore(); if err := store.Put(operational.Snapshot{ProviderName:"mock",Balance:100000,Health:operational.HealthUnhealthy}); err != nil { t.Fatal(err) }
 states := operational.NewProviderStateStore(); if err := states.Put(operational.ProviderState{ProviderName:"mock",Lifecycle:operational.LifecycleEnabled,Capabilities:[]operational.Capability{operational.CapabilityPPOB}}); err != nil { t.Fatal(err) }
 router, err := NewWithState(registry,store,nil,states); if err != nil { t.Fatal(err) }
 if _, err := router.Select(context.Background(),Request{ProductCode:"xld10",Amount:50000}); !errors.Is(err,ErrNoProviderAvailable) { t.Fatalf("expected unhealthy provider rejection, got %v",err) }
}

func TestRouterRejectsDegradedEnabledProvider(t *testing.T) {
 registry := provider.NewRegistry(); if err := registry.Register("mock",mock.New(mock.Config{Products:[]provider.Product{{Code:"xld10",Name:"Test"}}})); err != nil { t.Fatal(err) }
 store := operational.NewMemoryStore(); if err := store.Put(operational.Snapshot{ProviderName:"mock",Balance:100000,Health:operational.HealthDegraded}); err != nil { t.Fatal(err) }
 states := operational.NewProviderStateStore(); if err := states.Put(operational.ProviderState{ProviderName:"mock",Lifecycle:operational.LifecycleEnabled,Capabilities:[]operational.Capability{operational.CapabilityPPOB}}); err != nil { t.Fatal(err) }
 router, err := NewWithState(registry,store,nil,states); if err != nil { t.Fatal(err) }
 if _, err := router.Select(context.Background(),Request{ProductCode:"xld10",Amount:50000}); !errors.Is(err,ErrNoProviderAvailable) { t.Fatalf("expected degraded provider rejection, got %v",err) }
}

func TestRouterRejectsStaleCatalogForEnabledHealthyProvider(t *testing.T) {
 registry := provider.NewRegistry(); if err := registry.Register("mock",mock.New(mock.Config{Products:[]provider.Product{{Code:"xld10",Name:"Test"}}})); err != nil { t.Fatal(err) }
 store := operational.NewMemoryStore(); if err := store.Put(operational.Snapshot{ProviderName:"mock",Balance:100000,Health:operational.HealthHealthy}); err != nil { t.Fatal(err) }
 states := operational.NewProviderStateStore(); if err := states.Put(operational.ProviderState{ProviderName:"mock",Lifecycle:operational.LifecycleEnabled,Capabilities:[]operational.Capability{operational.CapabilityPPOB}}); err != nil { t.Fatal(err) }
 catalogs := catalog.NewMemoryStore(); if err := catalogs.Put(catalog.Snapshot{ProviderName:"mock",Products:[]provider.Product{{Code:"xld10",Name:"Test"}},SyncedAt:time.Now().Add(-2*time.Hour)}); err != nil { t.Fatal(err) }
 router, err := NewWithCatalogAndState(registry,store,nil,catalogs,states); if err != nil { t.Fatal(err) }
 if _, err := router.Select(context.Background(),Request{ProductCode:"xld10",Amount:50000}); !errors.Is(err,ErrNoProviderAvailable) { t.Fatalf("expected stale catalog rejection, got %v",err) }
}

func TestRouterSelectsEnabledHealthyFreshProviderWithState(t *testing.T) {
 registry := provider.NewRegistry(); if err := registry.Register("mock",mock.New(mock.Config{Products:[]provider.Product{{Code:"xld10",Name:"Test"}}})); err != nil { t.Fatal(err) }
 store := operational.NewMemoryStore(); if err := store.Put(operational.Snapshot{ProviderName:"mock",Balance:100000,Health:operational.HealthHealthy}); err != nil { t.Fatal(err) }
 states := operational.NewProviderStateStore(); if err := states.Put(operational.ProviderState{ProviderName:"mock",Lifecycle:operational.LifecycleEnabled,Capabilities:[]operational.Capability{operational.CapabilityPPOB}}); err != nil { t.Fatal(err) }
 catalogs := catalog.NewMemoryStore(); if err := catalogs.Put(catalog.Snapshot{ProviderName:"mock",Products:[]provider.Product{{Code:"xld10",Name:"Test"}},SyncedAt:time.Now()}); err != nil { t.Fatal(err) }
 router, err := NewWithCatalogAndState(registry,store,nil,catalogs,states); if err != nil { t.Fatal(err) }
 got, err := router.Select(context.Background(),Request{ProductCode:"xld10",Amount:50000}); if err != nil { t.Fatal(err) }; if got!="mock" { t.Fatalf("expected eligible provider, got %q",got) }
}


func TestRouterRestartRecoveryDoesNotTreatExpiredOperationalSnapshotAsRouteable(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       100000,
		Health:        operational.HealthHealthy,
		LastCheckedAt: now.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle:    operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}
	catalogs := catalog.NewMemoryStore()
	if err := catalogs.Put( catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10", Name: "Test"}},
		SyncedAt:     now,
	}); err != nil {
		t.Fatal(err)
	}

	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogs, states, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	router.Now = func() time.Time { return now }

	if _, ok := store.Get("mock"); !ok {
		t.Fatal("expected expired operational snapshot to remain available as persisted recovery data")
	}
	if _, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000}); !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expired recovery snapshot must not be routeable, got %v", err)
	}

	if err := store.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       125000,
		Health:        operational.HealthHealthy,
		LastCheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if got != "mock" {
		t.Fatalf("expected refreshed operational snapshot to become routeable, got %q", got)
	}
}

func TestRouterRejectsStaleOperationalSnapshot(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy, LastCheckedAt: time.Now().Add(-10 * time.Minute)}); err != nil { t.Fatal(err) }
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{ProviderName: "mock", Lifecycle: operational.LifecycleEnabled, Capabilities: []operational.Capability{operational.CapabilityPPOB}}); err != nil { t.Fatal(err) }
	catalogs := catalog.NewMemoryStore()
	if err := catalogs.Put(catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10", Name: "Test"}}, SyncedAt: time.Now()}); err != nil { t.Fatal(err) }
	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogs, states, time.Minute)
	if err != nil { t.Fatal(err) }
	if _, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000}); !errors.Is(err, ErrNoProviderAvailable) { t.Fatalf("expected stale operational snapshot rejection, got %v", err) }
}

func TestRouterSelectsFreshOperationalSnapshot(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy, LastCheckedAt: time.Now()}); err != nil { t.Fatal(err) }
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{ProviderName: "mock", Lifecycle: operational.LifecycleEnabled, Capabilities: []operational.Capability{operational.CapabilityPPOB}}); err != nil { t.Fatal(err) }
	catalogs := catalog.NewMemoryStore()
	if err := catalogs.Put(catalog.Snapshot{ProviderName: "mock", Products: []provider.Product{{Code: "xld10", Name: "Test"}}, SyncedAt: time.Now()}); err != nil { t.Fatal(err) }
	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogs, states, time.Minute)
	if err != nil { t.Fatal(err) }
	got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000})
	if err != nil { t.Fatal(err) }
	if got != "mock" { t.Fatalf("expected fresh provider selection, got %q", got) }
}

func TestNewWithCatalogAndStateAndOperationalMaxAgeRejectsInvalidMaxAge(t *testing.T) {
	registry := provider.NewRegistry()
	store := operational.NewMemoryStore()
	catalogs := catalog.NewMemoryStore()
	states := operational.NewProviderStateStore()
	if _, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogs, states, 0); err == nil { t.Fatal("expected invalid operational snapshot max age error") }
}


func TestRouterOperationalFreshnessAcceptsExactMaxAge(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       100000,
		Health:        operational.HealthHealthy,
		LastCheckedAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle:    operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}
	catalogs := catalog.NewMemoryStore()
	if err := catalogs.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10", Name: "Test"}},
		SyncedAt:     now,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogs, states, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	router.Now = func() time.Time { return now }
	if got, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000}); err != nil || got != "mock" {
		t.Fatalf("expected exact-age operational snapshot to remain eligible, got provider=%q err=%v", got, err)
	}
}

func TestRouterRejectsFutureOperationalSnapshot(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       100000,
		Health:        operational.HealthHealthy,
		LastCheckedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle:    operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}
	catalogs := catalog.NewMemoryStore()
	if err := catalogs.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10", Name: "Test"}},
		SyncedAt:     now,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogs, states, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	router.Now = func() time.Time { return now }
	if _, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000}); !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expected future operational snapshot rejection, got %v", err)
	}
}

func TestRouterRejectsFutureCatalogSnapshot(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	store := operational.NewMemoryStore()
	if err := store.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       100000,
		Health:        operational.HealthHealthy,
		LastCheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle:    operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}
	catalogs := catalog.NewMemoryStore()
	if err := catalogs.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10", Name: "Test"}},
		SyncedAt:     now.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogs, states, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	router.Now = func() time.Time { return now }
	if _, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 50000}); !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expected future catalog snapshot rejection, got %v", err)
	}
}
