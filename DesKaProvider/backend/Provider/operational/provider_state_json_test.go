package operational

import (
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
