package operational

import (
	"errors"
	"testing"
)

type failingProviderStatePersistence struct {
	states []ProviderState
	err    error
}

func (p *failingProviderStatePersistence) Load() ([]ProviderState, error) {
	return append([]ProviderState(nil), p.states...), nil
}

func (p *failingProviderStatePersistence) Save(states []ProviderState) error {
	p.states = append([]ProviderState(nil), states...)
	return p.err
}

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
		Capabilities: []Capability{CapabilityWebhook, CapabilityPPOB, CapabilityBalance, CapabilityCatalog, CapabilityPayout},
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
	if !got.Supports(CapabilityPPOB) || !got.Supports(CapabilityBalance) || !got.Supports(CapabilityWebhook) || !got.Supports(CapabilityCatalog) || !got.Supports(CapabilityPayout) {
		t.Fatalf("expected capabilities to be preserved: %#v", got.Capabilities)
	}
	if got.Supports(Capability("future")) {
		t.Fatal("unsupported capability must remain absent")
	}

	got.Capabilities[0] = Capability("future")
	again, _ := store.Get("mock")
	if again.Supports(Capability("future")) {
		t.Fatal("Get must return a defensive capability copy")
	}
}

func TestProviderStateStoreRejectsInvalidLifecycle(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{ProviderName: "mock", Lifecycle: Lifecycle("maintenance")}); err == nil {
		t.Fatal("expected invalid lifecycle error")
	}
}

func TestProviderStateStoreDoesNotCommitMemoryWhenPersistenceFails(t *testing.T) {
	persistErr := errors.New("persistence unavailable")
	persistence := &failingProviderStatePersistence{err: persistErr}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil {
		t.Fatal(err)
	}

	initial := ProviderState{
		ProviderName: "mock",
		Lifecycle:    LifecycleDisabled,
		Capabilities: []Capability{CapabilityPPOB},
	}
	persistence.err = nil
	if err := store.Put(initial); err != nil {
		t.Fatal(err)
	}

	persistence.err = persistErr
	updated := initial
	updated.Lifecycle = LifecycleEnabled
	updated.EnabledCapabilities = []Capability{CapabilityPPOB}
	if err := store.Put(updated); !errors.Is(err, persistErr) {
		t.Fatalf("expected persistence error, got %v", err)
	}

	got, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected existing state")
	}
	if got.Lifecycle != LifecycleDisabled {
		t.Fatalf("failed persistence must not commit lifecycle in memory: %q", got.Lifecycle)
	}
	if len(got.EnabledCapabilities) != 0 {
		t.Fatalf("failed persistence must not commit capability changes in memory: %#v", got.EnabledCapabilities)
	}
}
