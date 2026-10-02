package operational

import (
	"errors"
	"testing"
)

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


func TestProviderStateStoreSetLifecycleRejectsInvalidLifecycle(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleDisabled,
		Capabilities: []Capability{CapabilityPPOB},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.SetLifecycle("mock", Lifecycle("maintenance")); !errors.Is(err, ErrInvalidLifecycle) {
		t.Fatalf("expected ErrInvalidLifecycle, got %v", err)
	}
	state, ok := store.Get("mock")
	if !ok {
		t.Fatal("provider state disappeared")
	}
	if state.Lifecycle != LifecycleDisabled {
		t.Fatalf("invalid lifecycle mutation changed state: %#v", state)
	}
}


type ambiguousProviderStatePersistence struct {
	states []ProviderState
	err    error
}

func (p *ambiguousProviderStatePersistence) Load() ([]ProviderState, error) {
	return append([]ProviderState(nil), p.states...), nil
}

func (p *ambiguousProviderStatePersistence) Save(states []ProviderState) error {
	p.states = append([]ProviderState(nil), states...)
	return p.err
}

func TestProviderStateStoreAmbiguousLifecycleDisableFailsClosedInMemory(t *testing.T) {
	cause := errors.New("directory fsync failed")
	persistence := &ambiguousProviderStatePersistence{}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB},
	}); err != nil { t.Fatal(err) }

	persistence.err = errors.Join(ErrProviderStatePersistenceAmbiguous, cause)
	_, err = store.SetLifecycle("mock", LifecycleDisabled)
	if !errors.Is(err, ErrProviderStatePersistenceAmbiguous) || !errors.Is(err, cause) {
		t.Fatalf("expected ambiguous persistence and cause, got %v", err)
	}
	state, ok := store.Get("mock")
	if !ok { t.Fatal("provider state missing") }
	if state.Enabled() {
		t.Fatalf("ambiguous disable must fail closed in memory: %#v", state)
	}
}

func TestProviderStateStoreAmbiguousLifecycleEnableDoesNotPromote(t *testing.T) {
	cause := errors.New("directory fsync failed")
	persistence := &ambiguousProviderStatePersistence{}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleDisabled,
		Capabilities: []Capability{CapabilityPPOB},
	}); err != nil { t.Fatal(err) }

	persistence.err = errors.Join(ErrProviderStatePersistenceAmbiguous, cause)
	_, err = store.SetLifecycle("mock", LifecycleEnabled)
	if !errors.Is(err, ErrProviderStatePersistenceAmbiguous) || !errors.Is(err, cause) {
		t.Fatalf("expected ambiguous persistence and cause, got %v", err)
	}
	state, ok := store.Get("mock")
	if !ok { t.Fatal("provider state missing") }
	if state.Enabled() {
		t.Fatalf("ambiguous enable must remain fail-closed: %#v", state)
	}
}

func TestProviderStateStoreAmbiguousCapabilityDisableFailsClosed(t *testing.T) {
	cause := errors.New("directory fsync failed")
	persistence := &ambiguousProviderStatePersistence{}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB, CapabilityBalance},
		EnabledCapabilities: []Capability{CapabilityPPOB, CapabilityBalance},
	}); err != nil { t.Fatal(err) }

	persistence.err = errors.Join(ErrProviderStatePersistenceAmbiguous, cause)
	_, err = store.SetCapabilityEnabled("mock", CapabilityPPOB, false)
	if !errors.Is(err, ErrProviderStatePersistenceAmbiguous) || !errors.Is(err, cause) {
		t.Fatalf("expected ambiguous persistence and cause, got %v", err)
	}
	state, ok := store.Get("mock")
	if !ok { t.Fatal("provider state missing") }
	if state.Supports(CapabilityPPOB) {
		t.Fatalf("ambiguous capability disable must fail closed: %#v", state)
	}
	if !state.Supports(CapabilityBalance) {
		t.Fatalf("unrelated capability must remain enabled: %#v", state)
	}
}

func TestProviderStateStoreAmbiguousLegacyCapabilityMutationRemainsFailClosed(t *testing.T) {
	cause := errors.New("directory fsync failed")
	persistence := &ambiguousProviderStatePersistence{}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB, CapabilityBalance},
	}); err != nil { t.Fatal(err) }

	persistence.err = errors.Join(ErrProviderStatePersistenceAmbiguous, cause)
	_, err = store.SetCapabilityEnabled("mock", CapabilityPPOB, false)
	if !errors.Is(err, ErrProviderStatePersistenceAmbiguous) {
		t.Fatalf("expected ambiguous persistence, got %v", err)
	}
	state, ok := store.Get("mock")
	if !ok { t.Fatal("provider state missing") }
	if state.Supports(CapabilityPPOB) {
		t.Fatalf("legacy ambiguous capability disable must fail closed: %#v", state)
	}
	if !state.Supports(CapabilityBalance) {
		t.Fatalf("unrelated legacy capability must remain enabled: %#v", state)
	}
}


func TestProviderStateStoreReconcileCapabilityStatePrunesRemovedEnabledCapabilities(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB, CapabilityBalance},
		EnabledCapabilities: []Capability{CapabilityPPOB, CapabilityBalance},
		CapabilityFingerprint: "old",
	}); err != nil {
		t.Fatal(err)
	}

	state, err := store.ReconcileCapabilityState(
		"mock",
		[]Capability{CapabilityPPOB},
		"new",
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if state.Enabled() {
		t.Fatal("drift reconciliation must disable lifecycle")
	}
	if state.CapabilityFingerprint != "new" {
		t.Fatalf("unexpected fingerprint: %q", state.CapabilityFingerprint)
	}
	if state.Supports(CapabilityBalance) {
		t.Fatalf("removed capability remained route-eligible: %#v", state)
	}
	if !state.Supports(CapabilityPPOB) {
		t.Fatalf("implemented capability should remain enabled: %#v", state)
	}
}

func TestProviderStateStoreReconcileCapabilityStatePreservesConcurrentUnrelatedMutation(t *testing.T) {
	store := NewProviderStateStore()
	if err := store.Put(ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleDisabled,
		Capabilities: []Capability{CapabilityPPOB, CapabilityBalance},
		EnabledCapabilities: []Capability{CapabilityPPOB},
		CapabilityFingerprint: "old",
	}); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	done := make(chan error, 2)
	go func() {
		<-start
		_, err := store.ReconcileCapabilityState(
			"mock",
			[]Capability{CapabilityPPOB, CapabilityBalance},
			"new",
			false,
		)
		done <- err
	}()
	go func() {
		<-start
		var err error
		for i := 0; i < 32; i++ {
			_, err = store.SetCapabilityEnabled("mock", CapabilityBalance, true)
			if err != nil {
				break
			}
		}
		done <- err
	}()
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	state, ok := store.Get("mock")
	if !ok {
		t.Fatal("provider state missing")
	}
	if !state.Supports(CapabilityBalance) {
		t.Fatalf("concurrent capability mutation was lost: %#v", state)
	}
	if state.CapabilityFingerprint != "new" {
		t.Fatalf("reconciliation fingerprint was lost: %#v", state)
	}
}
