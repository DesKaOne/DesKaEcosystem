package operational

import "testing"

func TestProviderStateDefaultsDisabledAndNormalizesName(t *testing.T) {
	state, err := NewProviderState("  Mock ")
	if err != nil {
		t.Fatal(err)
	}
	if state.ProviderName != "mock" {
		t.Fatalf("unexpected provider name: %q", state.ProviderName)
	}
	if state.Lifecycle != LifecycleDisabled {
		t.Fatalf("expected disabled default, got %q", state.Lifecycle)
	}
	if state.Enabled() {
		t.Fatal("disabled provider must not be enabled")
	}
}

func TestProviderStateStoreSeparatesLifecycleFromCapabilities(t *testing.T) {
	store := NewProviderStateStore()
	state := ProviderState{
		ProviderName: "mock",
		Lifecycle:    LifecycleEnabled,
		Capabilities: []Capability{CapabilityWebhook, CapabilityPPOB, CapabilityBalance},
	}
	if err := store.Put(state); err != nil {
		t.Fatal(err)
	}

	got, ok := store.Get(" MOCK ")
	if !ok {
		t.Fatal("expected provider state")
	}
	if !got.Enabled() {
		t.Fatal("expected enabled lifecycle")
	}
	if !got.Supports(CapabilityPPOB) || !got.Supports(CapabilityBalance) || !got.Supports(CapabilityWebhook) {
		t.Fatalf("expected capabilities to be preserved: %#v", got.Capabilities)
	}
	if got.Supports(CapabilityPayout) {
		t.Fatal("unsupported capability must remain absent")
	}

	got.Capabilities[0] = CapabilityPayout
	again, _ := store.Get("mock")
	if again.Supports(CapabilityPayout) {
		t.Fatal("Get must return a defensive capability copy")
	}
}

func TestProviderStateStoreRejectsInvalidLifecycle(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{ProviderName: "mock", Lifecycle: Lifecycle("maintenance")}); err == nil {
		t.Fatal("expected invalid lifecycle error")
	}
}
