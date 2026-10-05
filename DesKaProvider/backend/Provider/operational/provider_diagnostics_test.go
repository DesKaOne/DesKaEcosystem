package operational

import (
	"reflect"
	"testing"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
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
		Capabilities: []Capability{CapabilityPPOB, CapabilityWebhook},
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
	if !state.Enabled() || len(state.Capabilities) != 2 {
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


func TestProviderAdminDiagnosticsDoNotMutateStateThroughReturnedCopies(t *testing.T) {
	registry := diagnosticRegistry(t)
	store := NewProviderStateStore()
	descriptor, err := registry.Capabilities("mock")
	if err != nil {
		t.Fatal(err)
	}
	original := ProviderState{
		ProviderName:          "mock",
		Lifecycle:             LifecycleEnabled,
		Capabilities:          []Capability{CapabilityPPOB, CapabilityWebhook},
		CapabilityFingerprint: CapabilityMetadataFingerprint(descriptor),
	}
	if err := store.Put(original); err != nil {
		t.Fatal(err)
	}

	admin, err := NewProviderAdminService(store)
	if err != nil {
		t.Fatal(err)
	}

	diagnostic, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic.State.Capabilities[0] = CapabilityPayment
	diagnostic.State.Capabilities = append(diagnostic.State.Capabilities, CapabilityPayout)

	current, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected provider state")
	}
	if !reflect.DeepEqual(current, original) {
		t.Fatalf("mutating Diagnose result must not mutate stored state: got %#v want %#v", current, original)
	}

	all, err := admin.DiagnoseAll(registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", all)
	}
	all[0].State.Capabilities[0] = CapabilityPayment

	current, ok = store.Get("mock")
	if !ok {
		t.Fatal("expected provider state")
	}
	if !reflect.DeepEqual(current, original) {
		t.Fatalf("mutating DiagnoseAll result must not mutate stored state: got %#v want %#v", current, original)
	}
}

func TestProviderAdminDiagnosticsRemainDeterministicAcrossStateTransitions(t *testing.T) {
	registry := diagnosticRegistry(t)
	store := NewProviderStateStore()
	descriptor, err := registry.Capabilities("mock")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := CapabilityMetadataFingerprint(descriptor)
	if err := store.Put(ProviderState{
		ProviderName:          "mock",
		Lifecycle:             LifecycleEnabled,
		Capabilities:          []Capability{CapabilityPPOB, CapabilityWebhook},
		CapabilityFingerprint: fingerprint,
	}); err != nil {
		t.Fatal(err)
	}

	admin, err := NewProviderAdminService(store)
	if err != nil {
		t.Fatal(err)
	}

	cleanA, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	cleanB, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cleanA, cleanB) {
		t.Fatalf("repeated clean diagnostics must be deterministic: %#v %#v", cleanA, cleanB)
	}

	driftedState, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected provider state")
	}
	driftedState.CapabilityFingerprint = "drifted"
	if err := store.Put(driftedState); err != nil {
		t.Fatal(err)
	}
	driftA, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	driftB, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(driftA, driftB) || !driftA.Drifted {
		t.Fatalf("repeated drift diagnostics must be deterministic: %#v %#v", driftA, driftB)
	}

	reconciledA, err := admin.ReconcileCapabilityState("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	reconciledB, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reconciledA, reconciledB) {
		t.Fatalf("reconciliation result and immediate diagnosis must agree: %#v %#v", reconciledA, reconciledB)
	}
	if reconciledA.Drifted || reconciledA.State.Enabled() {
		t.Fatalf("reconciliation must clear drift without enabling lifecycle: %#v", reconciledA)
	}

	if _, err := admin.Enable("mock"); err != nil {
		t.Fatal(err)
	}
	enabledA, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	enabledB, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(enabledA, enabledB) {
		t.Fatalf("repeated enabled diagnostics must be deterministic: %#v %#v", enabledA, enabledB)
	}
	if enabledA.Drifted || !enabledA.State.Enabled() {
		t.Fatalf("explicit enable should only restore lifecycle after reconciliation: %#v", enabledA)
	}
}
