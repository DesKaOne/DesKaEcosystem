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
