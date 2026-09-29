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

func TestRouterAggregateErrorsRemainDeterministicAcrossTransitions(t *testing.T) {
	now := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	registry := provider.NewRegistry()
	for _, name := range []string{"catalog-stale", "drift", "operational-stale"} {
		if err := registry.RegisterWithCapabilities(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		}), provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: {AdapterImplemented: true, Configured: true, Tested: true, Enabled: true},
		}}); err != nil {
			t.Fatal(err)
		}
	}

	states := operational.NewProviderStateStore()
	store := operational.NewMemoryStore()
	catalogs := catalog.NewMemoryStore()
	for _, name := range []string{"catalog-stale", "drift", "operational-stale"} {
		d, err := registry.Capabilities(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := states.Put(operational.ProviderState{
			ProviderName: name, Lifecycle: operational.LifecycleEnabled,
			Capabilities: []operational.Capability{operational.CapabilityPPOB},
			CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.Put(operational.Snapshot{
			ProviderName: name, Balance: 100000, Currency: "IDR",
			Health: operational.HealthHealthy, LastCheckedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		if err := catalogs.Put(catalog.Snapshot{
			ProviderName: name, Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}

	router, err := NewWithCatalogAndStateAndOperationalMaxAge(
		registry, store, nil, catalogs, states, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.CatalogMaxAge = time.Minute
	router.Now = func() time.Time { return now }

	join := func() error {
		_, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
		return err
	}
	assertAggregate := func(label string, err error, want ...error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: expected aggregate routing error", label)
		}
		if !errors.Is(err, ErrNoProviderAvailable) {
			t.Fatalf("%s: missing ErrNoProviderAvailable: %v", label, err)
		}
		for _, target := range want {
			if !errors.Is(err, target) {
				t.Fatalf("%s: missing %v: %v", label, target, err)
			}
		}
		const expectedOrder = "no provider available
provider operational snapshot is stale
provider catalog is stale
provider capability metadata drift detected"
		if len(want) == 3 && err.Error() != expectedOrder {
			t.Fatalf("%s: aggregate order changed: %q", label, err.Error())
		}
	}

	// Initial transition: all three failure causes coexist.
	drifted, ok := states.Get("drift")
	if !ok {
		t.Fatal("expected drift provider state")
	}
	drifted.CapabilityFingerprint = "drifted"
	if err := states.Put(drifted); err != nil {
		t.Fatal(err)
	}
	store.Put(operational.Snapshot{ProviderName: "operational-stale", Balance: 100000, Currency: "IDR", Health: operational.HealthHealthy, LastCheckedAt: now.Add(-2 * time.Minute)})
	catalogs.Put(catalog.Snapshot{ProviderName: "catalog-stale", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now.Add(-2 * time.Minute)})

	err = join()
	assertAggregate("all-blocking-causes", err, ErrOperationalSnapshotStale, ErrCatalogStale, ErrProviderCapabilityDrift)

	// Reconcile capability drift. Reconciliation must remove only the drift
	// sentinel; stale operational/catalog causes must remain.
	admin, err := operational.NewProviderAdminService(states)
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := admin.ReconcileCapabilityState("drift", registry)
	if err != nil || reconciled.Drifted {
		t.Fatalf("expected drift reconciliation, state=%#v err=%v", reconciled, err)
	}
	err = join()
	assertAggregate("after-drift-reconcile", err, ErrOperationalSnapshotStale, ErrCatalogStale)

	// Recover operational freshness. Only the catalog stale sentinel should
	// remain; aggregate membership must not retain historical causes.
	if err := store.Put(operational.Snapshot{
		ProviderName: "operational-stale", Balance: 100000, Currency: "IDR",
		Health: operational.HealthHealthy, LastCheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	err = join()
	assertAggregate("after-operational-recovery", err, ErrCatalogStale)

	// Recover catalog freshness. With every provider routeable, Select succeeds
	// and no historical aggregate error is retained.
	if err := catalogs.Put(catalog.Snapshot{
		ProviderName: "catalog-stale", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	selected, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
	if err != nil || selected == "" {
		t.Fatalf("expected routing recovery after all transitions, selected=%q err=%v", selected, err)
	}

	// Repeat the terminal state to prove aggregate recovery is deterministic.
	for i := 0; i < 3; i++ {
		selected, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
		if err != nil || selected == "" {
			t.Fatalf("terminal routing became non-deterministic on iteration %d: selected=%q err=%v", i, selected, err)
		}
	}
}
