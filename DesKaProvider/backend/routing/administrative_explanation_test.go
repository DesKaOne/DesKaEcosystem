package routing

import (
	"context"
	"reflect"
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
			Health: operational.HealthHealthy, LastCheckedAt: now,
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
