package operational

import (
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONFileProviderStateStorePersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider-state.json")
	persistence, err := NewJSONFileProviderStateStore(path)
	if err != nil { t.Fatal(err) }
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	state, _ := NewProviderState("Mock")
	state.Lifecycle = LifecycleEnabled
	state.Capabilities = []Capability{CapabilityWebhook, CapabilityPPOB}
	if err := store.Put(state); err != nil { t.Fatal(err) }

	reloadedPersistence, _ := NewJSONFileProviderStateStore(path)
	reloaded, err := NewPersistentProviderStateStore(reloadedPersistence)
	if err != nil { t.Fatal(err) }
	got, ok := reloaded.Get("mock")
	if !ok || !got.Enabled() || !got.Supports(CapabilityPPOB) || !got.Supports(CapabilityWebhook) { t.Fatalf("unexpected reloaded state: %#v", got) }
}

type failingProviderStatePersistence struct{}
func (failingProviderStatePersistence) Load() ([]ProviderState, error) { return nil, nil }
func (failingProviderStatePersistence) Save([]ProviderState) error { return errors.New("persistence failed") }

func TestProviderStateStoreDoesNotMutateMemoryWhenPersistenceFails(t *testing.T) {
	store, err := NewPersistentProviderStateStore(failingProviderStatePersistence{})
	if err != nil { t.Fatal(err) }
	state, _ := NewProviderState("mock")
	if err := store.Put(state); err == nil { t.Fatal("expected persistence error") }
	if _, ok := store.Get("mock"); ok { t.Fatal("failed persistent write must not mutate memory") }
}

func TestJSONFileProviderStateStoreCreatesSecureFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "provider-state.json")
	persistence, _ := NewJSONFileProviderStateStore(path)
	store, _ := NewPersistentProviderStateStore(persistence)
	state, _ := NewProviderState("mock")
	if err := store.Put(state); err != nil { t.Fatal(err) }
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0o600 { t.Fatalf("expected 0600, got %o", info.Mode().Perm()) }
}


func TestJSONFileProviderStateStorePreservesCapabilityFingerprintAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider-state.json")
	persistence, err := NewJSONFileProviderStateStore(path)
	if err != nil { t.Fatal(err) }

	descriptor := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true, Tested: true, Enabled: true},
	}}
	fingerprint := CapabilityMetadataFingerprint(descriptor)

	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	state := ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB},
		CapabilityFingerprint: fingerprint,
	}
	if err := store.Put(state); err != nil { t.Fatal(err) }

	reloadedPersistence, err := NewJSONFileProviderStateStore(path)
	if err != nil { t.Fatal(err) }
	reloaded, err := NewPersistentProviderStateStore(reloadedPersistence)
	if err != nil { t.Fatal(err) }
	got, ok := reloaded.Get("mock")
	if !ok {
		t.Fatal("expected persisted provider state")
	}
	if got.CapabilityFingerprint != fingerprint {
		t.Fatalf("capability fingerprint changed across restart: got=%q want=%q", got.CapabilityFingerprint, fingerprint)
	}
	if drift := DetectCapabilityDrift(got, descriptor); drift.Drifted() {
		t.Fatalf("matching recovered metadata must not drift: %#v", drift)
	}
}

func TestJSONFileProviderStateStoreRecoveryPreservesDriftEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider-state.json")
	persistence, err := NewJSONFileProviderStateStore(path)
	if err != nil { t.Fatal(err) }

	original := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true, Tested: true, Enabled: true},
	}}
	state := ProviderState{
		ProviderName: "mock",
		Lifecycle: LifecycleEnabled,
		Capabilities: []Capability{CapabilityPPOB},
		CapabilityFingerprint: CapabilityMetadataFingerprint(original),
	}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	if err := store.Put(state); err != nil { t.Fatal(err) }

	reloadedPersistence, err := NewJSONFileProviderStateStore(path)
	if err != nil { t.Fatal(err) }
	reloaded, err := NewPersistentProviderStateStore(reloadedPersistence)
	if err != nil { t.Fatal(err) }

	changed := provider.CapabilityDescriptor{Capabilities: map[provider.Capability]provider.CapabilityStatus{
		provider.CapabilityPPOB: {AdapterImplemented: true, Tested: true, Enabled: false},
	}}
	drift := DetectCapabilityDrift(func() ProviderState { value, _ := reloaded.Get("mock"); return value }(), changed)
	if !drift.Drifted() || !drift.MetadataChanged {
		t.Fatalf("recovered fingerprint must expose changed registry metadata: %#v", drift)
	}
	if drift.FingerprintUnavailable {
		t.Fatal("recovered fingerprint must remain available")
	}
}


type sequenceProviderStatePersistence struct {
	states   []ProviderState
	failSave bool
}

func (p *sequenceProviderStatePersistence) Load() ([]ProviderState, error) {
	states := make([]ProviderState, len(p.states))
	copy(states, p.states)
	return states, nil
}

func (p *sequenceProviderStatePersistence) Save(states []ProviderState) error {
	if p.failSave {
		return errors.New("persistence failed")
	}
	p.states = make([]ProviderState, len(states))
	copy(p.states, states)
	return nil
}

func TestProviderStateStorePersistsMultipleProvidersDeterministically(t *testing.T) {
	persistence := &sequenceProviderStatePersistence{}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }

	for _, name := range []string{"Zulu", "alpha", "midtrans"} {
		state, err := NewProviderState(name)
		if err != nil { t.Fatal(err) }
		state.Lifecycle = LifecycleEnabled
		state.Capabilities = []Capability{CapabilityPPOB}
		state.CapabilityFingerprint = name + "-fingerprint"
		if err := store.Put(state); err != nil { t.Fatal(err) }
	}

	persisted := persistence.states
	if len(persisted) != 3 {
		t.Fatalf("expected three persisted providers, got %d", len(persisted))
	}
	for i, want := range []string{"alpha", "midtrans", "zulu"} {
		if persisted[i].ProviderName != want {
			t.Fatalf("persisted provider order[%d] = %q, want %q", i, persisted[i].ProviderName, want)
		}
	}

	recovered, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }
	states := recovered.All()
	for i, want := range []string{"alpha", "midtrans", "zulu"} {
		if states[i].ProviderName != want {
			t.Fatalf("recovered provider order[%d] = %q, want %q", i, states[i].ProviderName, want)
		}
	}
	for _, state := range states {
		if state.CapabilityFingerprint != state.ProviderName+"-fingerprint" {
			t.Fatalf("recovered fingerprint for %q = %q", state.ProviderName, state.CapabilityFingerprint)
		}
	}
}

func TestProviderStateStoreFailedReplacementPreservesPreviousDurableState(t *testing.T) {
	persistence := &sequenceProviderStatePersistence{}
	store, err := NewPersistentProviderStateStore(persistence)
	if err != nil { t.Fatal(err) }

	original, _ := NewProviderState("mock")
	original.Lifecycle = LifecycleEnabled
	original.Capabilities = []Capability{CapabilityPPOB}
	original.CapabilityFingerprint = "original"
	if err := store.Put(original); err != nil { t.Fatal(err) }

	persistence.failSave = true
	replacement := original
	replacement.Lifecycle = LifecycleDisabled
	replacement.CapabilityFingerprint = "replacement"
	if err := store.Put(replacement); err == nil {
		t.Fatal("expected replacement persistence failure")
	}

	memory, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected existing in-memory state after failed replacement")
	}
	if memory.Lifecycle != LifecycleEnabled || memory.CapabilityFingerprint != "original" {
		t.Fatalf("failed replacement mutated memory: %#v", memory)
	}
	if len(persistence.states) != 1 || persistence.states[0].Lifecycle != LifecycleEnabled || persistence.states[0].CapabilityFingerprint != "original" {
		t.Fatalf("failed replacement mutated durable state: %#v", persistence.states)
	}
}
