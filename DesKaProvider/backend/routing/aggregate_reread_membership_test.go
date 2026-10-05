package routing

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
)

func TestAdministrativeReReadsPreserveExactRoutingAggregateMembership(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 30, 3, 0, 0, 0, time.UTC)

	registry := provider.NewRegistry()
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true, Configured: true, Tested: true, Enabled: true},
	}}
	for _, name := range []string{"catalog-stale", "drift", "operational-stale"} {
		if err := registry.RegisterWithCapabilities(name, mock.New(mock.Config{
			Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		}), descriptor); err != nil {
			t.Fatal(err)
		}
	}

	states := operational.NewProviderStateStore()
	operationalStore := operational.NewMemoryStore()
	catalogStore := catalog.NewMemoryStore()
	for _, name := range []string{"catalog-stale", "drift", "operational-stale"} {
		d, err := registry.Capabilities(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := states.Put(operational.ProviderState{
			ProviderName: name,
			Lifecycle: operational.LifecycleEnabled,
			Capabilities: []operational.Capability{operational.CapabilityPPOB},
			CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
		}); err != nil {
			t.Fatal(err)
		}

		lastCheckedAt := now
		if name == "operational-stale" {
			lastCheckedAt = now.Add(-2 * time.Minute)
		}
		if err := operationalStore.Put(operational.Snapshot{
			ProviderName: name, Balance: 100000, Currency: "IDR",
			Health: operational.HealthHealthy, LastCheckedAt: lastCheckedAt, LastSuccessAt: lastCheckedAt,
		}); err != nil {
			t.Fatal(err)
		}

		syncedAt := now
		if name == "catalog-stale" {
			syncedAt = now.Add(-2 * time.Minute)
		}
		if err := catalogStore.Put(catalog.Snapshot{
			ProviderName: name, Products: []provider.Product{{Code: "xld10", Name: "Test"}}, SyncedAt: syncedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}

	driftState, ok := states.Get("drift")
	if !ok {
		t.Fatal("expected drift provider state")
	}
	driftState.CapabilityFingerprint = "drifted"
	if err := states.Put(driftState); err != nil {
		t.Fatal(err)
	}

	router, err := NewWithCatalogAndStateAndOperationalMaxAge(
		registry, operationalStore, nil, catalogStore, states, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.CatalogMaxAge = time.Minute
	router.Now = func() time.Time { return now }

	admin, err := operational.NewProviderAdminService(states)
	if err != nil {
		t.Fatal(err)
	}

	assertAggregate := func(label, expected string, want ...error) {
		t.Helper()

		before, err := ExplainAllProviderRoutes(ctx, router)
		if err != nil {
			t.Fatal(err)
		}
		_, routeErr := router.Select(ctx, Request{ProductCode: "xld10", Amount: 100})
		if routeErr == nil {
			t.Fatalf("%s: expected blocked aggregate", label)
		}
		if routeErr.Error() != expected {
			t.Fatalf("%s: aggregate changed: got %q want %q", label, routeErr.Error(), expected)
		}
		if !errors.Is(routeErr, ErrNoProviderAvailable) {
			t.Fatalf("%s: missing ErrNoProviderAvailable: %v", label, routeErr)
		}
		for _, sentinel := range want {
			if !errors.Is(routeErr, sentinel) {
				t.Fatalf("%s: missing sentinel %v: %v", label, sentinel, routeErr)
			}
		}

		after, err := ExplainAllProviderRoutes(ctx, router)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("%s: administrative re-read mutated snapshot: before=%#v after=%#v", label, before, after)
		}

		for i := 0; i < 3; i++ {
			repeatBefore, err := ExplainAllProviderRoutes(ctx, router)
			if err != nil {
				t.Fatal(err)
			}
			_, repeatErr := router.Select(ctx, Request{ProductCode: "xld10", Amount: 100})
			if repeatErr == nil || repeatErr.Error() != expected {
				t.Fatalf("%s: repeated aggregate changed on iteration %d: %v", label, i, repeatErr)
			}
			if !errors.Is(repeatErr, ErrNoProviderAvailable) {
				t.Fatalf("%s: repeated aggregate lost ErrNoProviderAvailable on iteration %d: %v", label, i, repeatErr)
			}
			for _, sentinel := range want {
				if !errors.Is(repeatErr, sentinel) {
					t.Fatalf("%s: repeated aggregate lost %v on iteration %d: %v", label, sentinel, i, repeatErr)
				}
			}
			repeatAfter, err := ExplainAllProviderRoutes(ctx, router)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(repeatBefore, repeatAfter) {
				t.Fatalf("%s: repeated administrative re-read changed on iteration %d", label, i)
			}
			if repeatErr.Error() != routeErr.Error() {
				t.Fatalf("%s: aggregate accumulated or reordered on iteration %d: first=%q repeat=%q", label, i, routeErr.Error(), repeatErr.Error())
			}
		}
	}

	const allBlocked = "no provider available\nprovider operational snapshot is stale\nprovider catalog is stale\nprovider capability metadata drift detected"
	const noDrift = "no provider available\nprovider operational snapshot is stale\nprovider catalog is stale"
	const catalogOnly = "no provider available\nprovider catalog is stale"

	assertAggregate("initial-all-causes", allBlocked, ErrOperationalSnapshotStale, ErrCatalogStale, ErrProviderCapabilityDrift)

	reconciled, err := admin.ReconcileCapabilityState("drift", registry)
	if err != nil || reconciled.Drifted {
		t.Fatalf("expected drift reconciliation, state=%#v err=%v", reconciled, err)
	}
	assertAggregate("after-drift-reconcile", noDrift, ErrOperationalSnapshotStale, ErrCatalogStale)

	if err := operationalStore.Put(operational.Snapshot{
		ProviderName: "operational-stale", Balance: 100000, Currency: "IDR",
		Health: operational.HealthHealthy, LastCheckedAt: now, LastSuccessAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Disable("operational-stale"); err != nil {
		t.Fatal(err)
	}
	assertAggregate("after-operational-recovery-and-disable", catalogOnly, ErrCatalogStale)

	if err := catalogStore.Put(catalog.Snapshot{
		ProviderName: "catalog-stale", Products: []provider.Product{{Code: "xld10", Name: "Test"}}, SyncedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		before, err := ExplainAllProviderRoutes(ctx, router)
		if err != nil {
			t.Fatal(err)
		}
		selected, routeErr := router.Select(ctx, Request{ProductCode: "xld10", Amount: 100})
		if routeErr != nil || selected == "" {
			t.Fatalf("routing recovery failed on iteration %d: selected=%q err=%v", i, selected, routeErr)
		}
		if errors.Is(routeErr, ErrNoProviderAvailable) || errors.Is(routeErr, ErrCatalogStale) ||
			errors.Is(routeErr, ErrOperationalSnapshotStale) || errors.Is(routeErr, ErrProviderCapabilityDrift) {
			t.Fatalf("historical aggregate sentinel leaked into successful route on iteration %d: %v", i, routeErr)
		}
		after, err := ExplainAllProviderRoutes(ctx, router)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("successful routing re-read changed administrative snapshot on iteration %d", i)
		}
	}
}
