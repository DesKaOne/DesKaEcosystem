package routing

import (
	"context"
	"reflect"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
)

func TestExplainAllProviderRoutesDeterministicAndComplete(t *testing.T) {
	registry := provider.NewRegistry()
	status := provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true}
	names := []string{"midtrans", "iak", "xp-sindonesia", "digiflazz", "rcb-placeholder"}
	for _, name := range names {
		if err := registry.RegisterWithCapabilities(name, mock.New(mock.Config{}), provider.CapabilityDescriptor{
			Capabilities: map[provider.Capability]provider.CapabilityStatus{
				provider.CapabilityPPOB: status,
			},
		}); err != nil { t.Fatal(err) }
	}
	states := operational.NewProviderStateStore()
	for _, name := range names {
		d, err := registry.Capabilities(name)
		if err != nil { t.Fatal(err) }
		if err := states.Put(operational.ProviderState{
			ProviderName: name, Lifecycle: operational.LifecycleDisabled,
			Capabilities: []operational.Capability{operational.CapabilityPPOB},
			CapabilityFingerprint: operational.CapabilityMetadataFingerprint(d),
		}); err != nil { t.Fatal(err) }
	}
	store := operational.NewMemoryStore()
	now := time.Now()
	for _, name := range names {
		if err := store.Put(operational.Snapshot{
			ProviderName: name, Balance: 100000, Currency: "IDR",
			Health: operational.HealthHealthy, LastCheckedAt: now, LastSuccessAt: now,
		}); err != nil { t.Fatal(err) }
	}
	catalogStore := catalog.NewMemoryStore()
	for _, name := range names {
		if err := catalogStore.Put(catalog.Snapshot{
			ProviderName: name, Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now,
		}); err != nil { t.Fatal(err) }
	}
	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, store, nil, catalogStore, states, time.Hour)
	if err != nil { t.Fatal(err) }
	fixedNow := now.Add(30 * time.Second)
	router.Now = func() time.Time { return fixedNow }
	first, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil { t.Fatal(err) }
	second, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(first, second) { t.Fatalf("snapshot is not deterministic: first=%#v second=%#v", first, second) }
	expectedNames := []string{"digiflazz", "iak", "midtrans", "rcb-placeholder", "xp-sindonesia"}
	if len(first.Providers) != len(expectedNames) { t.Fatalf("expected %d providers, got %d", len(expectedNames), len(first.Providers)) }
	for i, name := range expectedNames {
		if first.Providers[i].ProviderName != name { t.Fatalf("provider order is not deterministic: %#v", first.Providers) }
		if len(first.Providers[i].Capabilities) != len(provider.AllCapabilities()) { t.Fatalf("provider %q lacks full capability coverage: %#v", name, first.Providers[i]) }
	}
}


func TestExplainAllProviderRoutesAuditsFreshnessAndDrift(t *testing.T) {
	registry := provider.NewRegistry()
	status := provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true}
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{}), provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: status,
		},
	}); err != nil { t.Fatal(err) }

	states := operational.NewProviderStateStore()
	descriptor, err := registry.Capabilities("mock")
	if err != nil { t.Fatal(err) }
	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle: operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: operational.CapabilityMetadataFingerprint(descriptor),
	}); err != nil { t.Fatal(err) }

	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance: 1000,
		Currency: "IDR",
		Health: operational.HealthHealthy,
		LastCheckedAt: now.Add(-30 * time.Second),
	}); err != nil { t.Fatal(err) }

	catalogStore := catalog.NewMemoryStore()
	if err := catalogStore.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products: []provider.Product{{Code: "xld10"}},
		SyncedAt: now.Add(-45 * time.Second),
	}); err != nil { t.Fatal(err) }

	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, operationalStore, nil, catalogStore, states, time.Minute)
	if err != nil { t.Fatal(err) }
	router.CatalogMaxAge = time.Minute
	router.Now = func() time.Time { return now }

	snapshot, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil { t.Fatal(err) }
	if !snapshot.GeneratedAt.Equal(now) { t.Fatalf("unexpected snapshot generation time: %v", snapshot.GeneratedAt) }
	if len(snapshot.Providers) != 1 { t.Fatalf("expected one provider, got %d", len(snapshot.Providers)) }
	freshness := snapshot.Providers[0].Freshness
	if !freshness.OperationalPresent || !freshness.OperationalFresh {
		t.Fatalf("expected fresh operational source metadata: %#v", freshness)
	}
	if !freshness.CatalogPresent || !freshness.CatalogFresh {
		t.Fatalf("expected fresh catalog source metadata: %#v", freshness)
	}
	if freshness.CapabilityDrifted {
		t.Fatalf("expected synchronized capability metadata: %#v", freshness)
	}

	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle: operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: "stale-fingerprint",
	}); err != nil { t.Fatal(err) }

	snapshot, err = ExplainAllProviderRoutes(context.Background(), router)
	if err != nil { t.Fatal(err) }
	if !snapshot.Providers[0].Freshness.CapabilityDrifted {
		t.Fatal("expected capability drift to be auditable")
	}
}

func TestExplainAllProviderRoutesReconstructsFromPersistentSources(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "provider-state.json")
	operationalPath := filepath.Join(dir, "operational.json")
	catalogPath := filepath.Join(dir, "catalog.json")

	registry := provider.NewRegistry()
	status := provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true}
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{}), provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: status,
		},
	}); err != nil { t.Fatal(err) }
	descriptor, err := registry.Capabilities("mock")
	if err != nil { t.Fatal(err) }

	statePersistence, err := operational.NewJSONFileProviderStateStore(statePath)
	if err != nil { t.Fatal(err) }
	stateStore, err := operational.NewPersistentProviderStateStore(statePersistence)
	if err != nil { t.Fatal(err) }
	if err := stateStore.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle: operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: operational.CapabilityMetadataFingerprint(descriptor),
	}); err != nil { t.Fatal(err) }

	operationalStore, err := operational.NewJSONFileStore(operationalPath)
	if err != nil { t.Fatal(err) }
	catalogStore, err := catalog.NewJSONFileStore(catalogPath)
	if err != nil { t.Fatal(err) }
	now := time.Date(2026, 9, 29, 13, 0, 0, 0, time.UTC)
	if err := operationalStore.Put(operational.Snapshot{
		ProviderName: "mock", Balance: 1000, Currency: "IDR",
		Health: operational.HealthHealthy, LastCheckedAt: now, LastSuccessAt: now,
	}); err != nil { t.Fatal(err) }
	if err := catalogStore.Put(catalog.Snapshot{
		ProviderName: "mock", Products: []provider.Product{{Code: "xld10"}}, SyncedAt: now,
	}); err != nil { t.Fatal(err) }

	// Reconstruct every source from disk to model a runtime restart.
	recoveredStatePersistence, err := operational.NewJSONFileProviderStateStore(statePath)
	if err != nil { t.Fatal(err) }
	recoveredState, err := operational.NewPersistentProviderStateStore(recoveredStatePersistence)
	if err != nil { t.Fatal(err) }
	recoveredOperational, err := operational.NewJSONFileStore(operationalPath)
	if err != nil { t.Fatal(err) }
	recoveredCatalog, err := catalog.NewJSONFileStore(catalogPath)
	if err != nil { t.Fatal(err) }

	router, err := NewWithCatalogAndStateAndOperationalMaxAge(registry, recoveredOperational, nil, recoveredCatalog, recoveredState, time.Minute)
	if err != nil { t.Fatal(err) }
	router.Now = func() time.Time { return now }
	router.CatalogMaxAge = time.Minute

	snapshot, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil { t.Fatal(err) }
	if !snapshot.GeneratedAt.Equal(now) { t.Fatalf("unexpected generation time after restart: %v", snapshot.GeneratedAt) }
	if len(snapshot.Providers) != 1 { t.Fatalf("expected one recovered provider, got %d", len(snapshot.Providers)) }
	freshness := snapshot.Providers[0].Freshness
	if !freshness.OperationalPresent || !freshness.OperationalFresh || !freshness.OperationalLastCheckedAt.Equal(now) {
		t.Fatalf("operational freshness did not recover: %#v", freshness)
	}
	if !freshness.CatalogPresent || !freshness.CatalogFresh || !freshness.CatalogSyncedAt.Equal(now) {
		t.Fatalf("catalog freshness did not recover: %#v", freshness)
	}
	if freshness.CapabilityDrifted {
		t.Fatalf("capability drift should remain clear after restart: %#v", freshness)
	}
}
