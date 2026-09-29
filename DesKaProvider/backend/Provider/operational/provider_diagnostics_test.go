package operational

import (
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/mock"
)

func diagnosticRegistry(t *testing.T) *provider.Registry {
	t.Helper()
	registry := provider.NewRegistry()
	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true, Tested: true, Enabled: true},
		provider.CapabilityWebhook: {AdapterImplemented: true, Tested: true, Enabled: false},
	}}
	if err := registry.RegisterWithCapabilities("mock", mock.New(mock.Config{}), descriptor); err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestProviderAdminDiagnoseIsObservational(t *testing.T) {
	registry := diagnosticRegistry(t)
	store := NewProviderStateStore()
	descriptor, err := registry.Capabilities("mock")
	if err != nil { t.Fatal(err) }
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB},
		CapabilityFingerprint: CapabilityMetadataFingerprint(descriptor),
	}); err != nil { t.Fatal(err) }

	admin, err := NewProviderAdminService(store)
	if err != nil { t.Fatal(err) }
	diagnostic, err := admin.Diagnose("mock", registry)
	if err != nil { t.Fatal(err) }
	if diagnostic.Drifted || diagnostic.Drift.Drifted() {
		t.Fatalf("unexpected drift: %#v", diagnostic)
	}
	state, _ := store.Get("mock")
	if !state.Enabled() || len(state.Capabilities) != 1 {
		t.Fatalf("diagnose must not mutate state: %#v", state)
	}
}

func TestProviderAdminReconcileDisablesOnDriftAndRequiresExplicitEnable(t *testing.T) {
	registry := diagnosticRegistry(t)
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB},
		CapabilityFingerprint: "stale",
	}); err != nil { t.Fatal(err) }

	admin, err := NewProviderAdminService(store)
	if err != nil { t.Fatal(err) }
	diagnostic, err := admin.ReconcileCapabilityState("mock", registry)
	if err != nil { t.Fatal(err) }
	if diagnostic.Drifted {
		t.Fatalf("reconciled state should be clean: %#v", diagnostic)
	}
	if diagnostic.State.Enabled() {
		t.Fatal("reconciliation must not auto-enable a drifted provider")
	}
	if !diagnostic.State.Supports(CapabilityWebhook) {
		t.Fatalf("reconciliation must synchronize implemented capabilities: %#v", diagnostic.State.Capabilities)
	}

	enabled, err := admin.Enable("mock")
	if err != nil { t.Fatal(err) }
	if !enabled.Enabled() {
		t.Fatal("explicit enable must restore only the lifecycle gate")
	}
	if enabled.CapabilityFingerprint != diagnostic.State.CapabilityFingerprint {
		t.Fatal("explicit enable must preserve reconciled capability fingerprint")
	}
}

func TestProviderAdminDiagnoseAllIsDeterministic(t *testing.T) {
	registry := provider.NewRegistry()
	for _, name := range []string{"zeta", "alpha"} {
		if err := registry.RegisterWithCapabilities(name, mock.New(mock.Config{}), provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
			provider.CapabilityPPOB: {AdapterImplemented: true},
		}}); err != nil { t.Fatal(err) }
	}
	store := NewProviderStateStore()
	for _, name := range []string{"zeta", "alpha"} {
		if err := store.Put(ProviderState{ProviderName: name, Lifecycle: LifecycleDisabled, Capabilities: []Capability{CapabilityPPOB}}); err != nil { t.Fatal(err) }
	}
	admin, err := NewProviderAdminService(store)
	if err != nil { t.Fatal(err) }
	diagnostics, err := admin.DiagnoseAll(registry)
	if err != nil { t.Fatal(err) }
	if len(diagnostics) != 2 || diagnostics[0].State.ProviderName != "alpha" || diagnostics[1].State.ProviderName != "zeta" {
		t.Fatalf("diagnostics must be deterministic: %#v", diagnostics)
	}
}
