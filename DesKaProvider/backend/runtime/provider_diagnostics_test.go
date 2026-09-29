package runtime

import (
	"context"
	"errors"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

func TestProviderDiagnosticsRecoveryKeepsReadinessSeparate(t *testing.T) {
	registry := provider.NewRegistry()
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {Verified: true, Configured: true, AdapterImplemented: true, Tested: true, Enabled: true, LiveTested: false},
	}}
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), descriptor); err != nil {
		t.Fatal(err)
	}
	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{ProviderName: "mock", Balance: 100000, Health: operational.HealthHealthy}); err != nil {
		t.Fatal(err)
	}
	states := operational.NewProviderStateStore()
	if err := states.Put(operational.ProviderState{
		ProviderName: "mock",
		Lifecycle: operational.LifecycleEnabled,
		Capabilities: []operational.Capability{operational.CapabilityPPOB},
		CapabilityFingerprint: "stale",
	}); err != nil {
		t.Fatal(err)
	}
	router, err := routing.NewWithState(registry, operationalStore, nil, states)
	if err != nil { t.Fatal(err) }
	purchase, err := routing.NewService(router)
	if err != nil { t.Fatal(err) }
	service := &Service{providerState: states, purchaseService: purchase}

	diagnostics, err := service.ProviderDiagnostics()
	if err != nil { t.Fatal(err) }
	if len(diagnostics) != 1 || !diagnostics[0].Drifted {
		t.Fatalf("expected observable drift, got %#v", diagnostics)
	}

	reconciled, err := service.ReconcileProviderCapabilities("mock")
	if err != nil { t.Fatal(err) }
	if reconciled.Drifted || reconciled.State.Enabled() {
		t.Fatalf("reconciliation must clear drift without enabling lifecycle: %#v", reconciled)
	}

	if _, err := router.Select(context.Background(), routing.Request{ProductCode: "xld10", Amount: 1000}); !errors.Is(err, routing.ErrNoProviderAvailable) {
		t.Fatalf("reconciled-but-disabled provider must remain non-routable, got %v", err)
	}

	enabled, err := service.EnableProvider("mock")
	if err != nil { t.Fatal(err) }
	if !enabled.Enabled() {
		t.Fatal("explicit enable should restore lifecycle")
	}
	if selected, err := router.Select(context.Background(), routing.Request{ProductCode: "xld10", Amount: 1000}); err != nil || selected != "mock" {
		t.Fatalf("explicitly re-enabled and reconciled provider should become route-eligible, selected=%q err=%v", selected, err)
	}

	// Explicit lifecycle recovery must not promote live validation or ProductionReady.
	status, ok := descriptor.Status(provider.CapabilityPPOB)
	if !ok || status.LiveTested || status.ProductionReady {
		t.Fatalf("test fixture readiness unexpectedly changed: %+v", status)
	}
	current, err := registry.Capabilities("mock")
	if err != nil { t.Fatal(err) }
	currentStatus, _ := current.Status(provider.CapabilityPPOB)
	if currentStatus.LiveTested || currentStatus.ProductionReady {
		t.Fatalf("runtime recovery must not promote capability readiness: %+v", currentStatus)
	}
}
