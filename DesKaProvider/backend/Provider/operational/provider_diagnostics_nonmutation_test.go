package operational

import (
	"reflect"
	"testing"

)

func TestProviderAdminDiagnosticsReturnedDriftCopiesAreIsolated(t *testing.T) {
	registry := diagnosticRegistry(t)
	store := NewProviderStateStore()

	descriptor, err := registry.Capabilities("mock")
	if err != nil {
		t.Fatal(err)
	}
	original := ProviderState{
		ProviderName:          "mock",
		Lifecycle:             LifecycleEnabled,
		Capabilities:          []Capability{CapabilityPPOB},
		CapabilityFingerprint: CapabilityMetadataFingerprint(descriptor),
	}
	if err := store.Put(original); err != nil {
		t.Fatal(err)
	}

	admin, err := NewProviderAdminService(store)
	if err != nil {
		t.Fatal(err)
	}

	first, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Drift.Added) == 0 {
		t.Fatalf("expected drift fixture to expose an added capability: %#v", first.Drift)
	}

	first.Drift.Added[0] = CapabilityPayment
	first.Drift.Added = append(first.Drift.Added, CapabilityPayout)
	first.State.Capabilities[0] = CapabilityPayment

	all, err := admin.DiagnoseAll(registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", all)
	}
	if len(all[0].Drift.Added) == 0 {
		t.Fatalf("expected DiagnoseAll drift evidence: %#v", all[0].Drift)
	}
	all[0].Drift.Added[0] = CapabilityPayment
	all[0].State.Capabilities[0] = CapabilityPayment

	current, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected persisted provider state")
	}
	if !reflect.DeepEqual(current, original) {
		t.Fatalf("diagnostic result mutation changed provider state: got %#v want %#v", current, original)
	}

	repeated, err := admin.Diagnose("mock", registry)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repeated, expected) {
		t.Fatalf("diagnostics must be deterministic and isolated from caller mutation: expected=%#v repeated=%#v", expected, repeated)
	}
}
