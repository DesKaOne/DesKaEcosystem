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

func TestAdministrativeDiagnosticsDoNotMutateRoutingSources(t *testing.T) {
	registry := provider.NewRegistry()
	status := provider.CapabilityStatus{AdapterImplemented: true, Enabled: true, Tested: true}
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{
		Products: []provider.Product{{Code: "xld10"}},
	}), provider.CapabilityDescriptor{
		Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: status,
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
		ProviderName: "mock",
		Lifecycle: operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: operational.CapabilityMetadataFingerprint(descriptor),
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 29, 20, 0, 0, 0, time.UTC)
	operationalStore := operational.NewMemoryStore()
	operationalSnapshot := operational.Snapshot{
		ProviderName: "mock",
		Balance: 100000,
		Currency: "IDR",
		Health: operational.HealthHealthy,
		LastCheckedAt: now,
		LastSuccessAt: now,
	}
	if err := operationalStore.Put(operationalSnapshot); err != nil {
		t.Fatal(err)
	}

	catalogStore := catalog.NewMemoryStore()
	catalogSnapshot := catalog.Snapshot{
		ProviderName: "mock",
		Products: []provider.Product{{Code: "xld10", Name: "Test"}},
		SyncedAt: now,
	}
	if err := catalogStore.Put(catalogSnapshot); err != nil {
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
		t.Fatal("expected provider state")
	}
	beforeOperational, ok := operationalStore.Get("mock")
	if !ok {
		t.Fatal("expected operational snapshot")
	}
	beforeCatalog, ok := catalogStore.Get("mock")
	if !ok {
		t.Fatal("expected catalog snapshot")
	}
	beforeDescriptor, err := registry.Capabilities("mock")
	if err != nil {
		t.Fatal(err)
	}

	beforeRoute, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
	if err != nil || beforeRoute != "mock" {
		t.Fatalf("expected eligible route before diagnostics, provider=%q err=%v", beforeRoute, err)
	}

	explanation, err := ExplainProviderRoute(context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(explanation.Reasons) == 0 {
		t.Fatal("expected readiness diagnostics in explanation fixture")
	}
	explanation.Reasons[0] = ReadinessReason{Code: ReasonProviderNotRegistered, Blocking: true}
	explanation.Reasons = append(explanation.Reasons, ReadinessReason{Code: ReasonCatalogStale, Blocking: true})

	snapshot, err := ExplainAllProviderRoutes(context.Background(), router)
	if err != nil {
		t.Fatal(err)
	}
	foundReason := false
	for i := range snapshot.Providers {
		for j := range snapshot.Providers[i].Capabilities {
			if len(snapshot.Providers[i].Capabilities[j].Reasons) > 0 {
				snapshot.Providers[i].Capabilities[j].Reasons[0] = ReadinessReason{Code: ReasonProviderNotRegistered, Blocking: true}
				snapshot.Providers[i].Capabilities[j].Reasons = append(
					snapshot.Providers[i].Capabilities[j].Reasons,
					ReadinessReason{Code: ReasonCatalogStale, Blocking: true},
				)
				foundReason = true
				break
			}
		}
		if foundReason {
			break
		}
	}
	if !foundReason {
		t.Fatal("expected at least one aggregate explanation with reasons")
	}

	afterState, _ := states.Get("mock")
	afterOperational, _ := operationalStore.Get("mock")
	afterCatalog, _ := catalogStore.Get("mock")
	afterDescriptor, _ := registry.Capabilities("mock")

	if !reflect.DeepEqual(afterState, beforeState) {
		t.Fatalf("diagnostics mutated provider state: got %#v want %#v", afterState, beforeState)
	}
	if !reflect.DeepEqual(afterOperational, beforeOperational) {
		t.Fatalf("diagnostics mutated operational snapshot: got %#v want %#v", afterOperational, beforeOperational)
	}
	if !reflect.DeepEqual(afterCatalog, beforeCatalog) {
		t.Fatalf("diagnostics mutated catalog snapshot: got %#v want %#v", afterCatalog, beforeCatalog)
	}
	if !reflect.DeepEqual(afterDescriptor, beforeDescriptor) {
		t.Fatalf("diagnostics mutated capability metadata: got %#v want %#v", afterDescriptor, beforeDescriptor)
	}

	afterRoute, err := router.Select(context.Background(), Request{ProductCode: "xld10", Amount: 100})
	if err != nil || afterRoute != beforeRoute {
		t.Fatalf("diagnostics changed routing outcome: before=%q after=%q err=%v", beforeRoute, afterRoute, err)
	}

	repeated, err := ExplainProviderRoute(context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ExplainProviderRoute(context.Background(), router, "mock", provider.CapabilityPPOB, "xld10", 100)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repeated, again) {
		t.Fatalf("repeated explanations must remain deterministic: first=%#v second=%#v", repeated, again)
	}
}
