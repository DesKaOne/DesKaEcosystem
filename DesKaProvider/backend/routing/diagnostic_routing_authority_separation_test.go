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

func TestAdministrativeDiagnosticsRemainObservationalAndSeparateFromRoutingAuthority(t *testing.T) {
	now := time.Date(2026, 9, 30, 1, 0, 0, 0, time.UTC)
	registry := provider.NewRegistry()
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{
		Products: []provider.Product{{Code: "xld10", Name: "Test"}},
	}), provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: {
				AdapterImplemented: true,
				Configured:         true,
				Tested:             true,
				Enabled:            true,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	states := operational.NewProviderStateStore()
	descriptor, err := registry.Capabilities("mock")
	if err != nil {
		t.Fatal(err)
	}
	if err := states.Put(operational.ProviderState{
		ProviderName:          "mock",
		Lifecycle:             operational.LifecycleDisabled,
		Capabilities:          []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: operational.CapabilityMetadataFingerprint(descriptor),
	}); err != nil {
		t.Fatal(err)
	}

	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{
		ProviderName:  "mock",
		Balance:       100000,
		Currency:      "IDR",
		Health:        operational.HealthHealthy,
		LastCheckedAt: now.Add(-2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	catalogStore := catalog.NewMemoryStore()
	if err := catalogStore.Put(catalog.Snapshot{
		ProviderName: "mock",
		Products:     []provider.Product{{Code: "xld10"}},
		SyncedAt:     now.Add(-2 * time.Minute),
	}); err != nil {
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

	beforeState, ok := states.Get("mock")
	if !ok {
		t.Fatal("expected persisted provider state")
	}
	beforeOperational, ok := operationalStore.Get("mock")
	if !ok {
		t.Fatal("expected operational snapshot")
	}
	beforeCatalog, ok := catalogStore.Get("mock")
	if !ok {
		t.Fatal("expected catalog snapshot")
	}

	_, err = router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
	if err == nil {
		t.Fatal("expected routing to remain blocked")
	}
	selectBeforeErr := err.Error()
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("expected no-provider sentinel, got %v", err)
	}
	if errors.Is(err, ErrOperationalSnapshotStale) || errors.Is(err, ErrCatalogStale) {
		t.Fatalf("disabled lifecycle must prevent stale-source sentinels from becoming routing causes: %v", err)
	}
	if selectBeforeErr != ErrNoProviderAvailable.Error() {
		t.Fatalf("unexpected authoritative routing error: %q", selectBeforeErr)
	}

	first, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated administrative explanation changed: first=%#v second=%#v", first, second)
	}

	if len(first.Providers) != 1 {
		t.Fatalf("expected one provider explanation, got %d", len(first.Providers))
	}
	var ppob *ProviderCapabilityRouteExplanation
	for i := range first.Providers[0].Capabilities {
		if first.Providers[0].Capabilities[i].Capability == provider.CapabilityPPOB {
			ppob = &first.Providers[0].Capabilities[i]
			break
		}
	}
	if ppob == nil {
		t.Fatal("expected PPOB administrative explanation")
	}
	if ppob.RouteEligible {
		t.Fatal("disabled provider must remain non-route-eligible")
	}
	reasons := map[ReadinessReasonCode]bool{}
	for _, reason := range ppob.Reasons {
		reasons[reason.Code] = true
	}
	for _, code := range []ReadinessReasonCode{
		ReasonLifecycleDisabled,
		ReasonOperationalSnapshotStale,
		ReasonCatalogStale,
	} {
		if !reasons[code] {
			t.Fatalf("administrative explanation did not expose diagnostic evidence %q: %#v", code, ppob.Reasons)
		}
	}

	afterState, ok := states.Get("mock")
	if !ok {
		t.Fatal("provider state disappeared after diagnostics")
	}
	afterOperational, ok := operationalStore.Get("mock")
	if !ok {
		t.Fatal("operational snapshot disappeared after diagnostics")
	}
	afterCatalog, ok := catalogStore.Get("mock")
	if !ok {
		t.Fatal("catalog snapshot disappeared after diagnostics")
	}
	if !reflect.DeepEqual(beforeState, afterState) {
		t.Fatalf("administrative explanation mutated provider lifecycle/capability state: before=%#v after=%#v", beforeState, afterState)
	}
	if !reflect.DeepEqual(beforeOperational, afterOperational) {
		t.Fatalf("administrative explanation mutated operational state: before=%#v after=%#v", beforeOperational, afterOperational)
	}
	if !reflect.DeepEqual(beforeCatalog, afterCatalog) {
		t.Fatalf("administrative explanation mutated catalog state: before=%#v after=%#v", beforeCatalog, afterCatalog)
	}

	_, err = router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
	if err == nil {
		t.Fatal("expected routing to remain blocked after diagnostics")
	}
	selectAfterErr := err.Error()
	if selectAfterErr != selectBeforeErr {
		t.Fatalf("administrative diagnostics altered authoritative routing error: before=%q after=%q", selectBeforeErr, selectAfterErr)
	}
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Fatalf("routing lost no-provider sentinel after diagnostics: %v", err)
	}
}
