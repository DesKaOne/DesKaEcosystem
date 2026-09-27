package catalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestJSONFileStorePersistsAndReloadsCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog", "state.json")
	now := time.Date(2026,9,25,12,0,0,0,time.UTC)
	store, err := NewJSONFileStore(path)
	if err != nil { t.Fatal(err) }
	if err := store.Put(Snapshot{ProviderName:"mock", Products:[]provider.Product{{Code:"xld10",Name:"XL 10K"}}, SyncedAt:now}); err != nil { t.Fatal(err) }
	reloaded, err := NewJSONFileStore(path)
	if err != nil { t.Fatal(err) }
	got, ok := reloaded.Get("mock")
	if !ok || got.SyncedAt != now || len(got.Products) != 1 || got.Products[0].Code != "xld10" { t.Fatalf("unexpected reloaded snapshot: %#v", got) }
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0600 { t.Fatalf("expected 0600 store permissions, got %o", info.Mode().Perm()) }
}

func TestJSONFileStoreRejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil { t.Fatal(err) }
	if _, err := NewJSONFileStore(path); err == nil { t.Fatal("expected corrupt JSON error") }
}

func TestJSONFileStoreRejectsReadErrorInsteadOfTreatingPathAsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog-dir")
	if err := os.Mkdir(path, 0750); err != nil { t.Fatal(err) }
	if _, err := NewJSONFileStore(path); err == nil { t.Fatal("expected read error for directory path") }
}

func TestJSONFileStoreRejectsSemanticallyInvalidSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	payload := []byte(`{"snapshots":{"mock":{"provider_name":"","products":[],"synced_at":"0001-01-01T00:00:00Z"}}}`)
	if err := os.WriteFile(path, payload, 0600); err != nil { t.Fatal(err) }
	if _, err := NewJSONFileStore(path); err == nil { t.Fatal("expected semantically invalid snapshot error") }
}


func TestJSONFileStoreRejectsOlderSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	newer := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Minute)
	store, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(Snapshot{ProviderName: "mock", SyncedAt: newer}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(Snapshot{ProviderName: "mock", SyncedAt: older}); err != ErrSnapshotOlder {
		t.Fatalf("expected older snapshot rejection, got %v", err)
	}
	got, ok := store.Get("mock")
	if !ok || !got.SyncedAt.Equal(newer) {
		t.Fatalf("older snapshot replaced current state: %#v", got)
	}
	reloaded, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok = reloaded.Get("mock")
	if !ok || !got.SyncedAt.Equal(newer) {
		t.Fatalf("older snapshot was persisted after rejection: %#v", got)
	}
}


func TestJSONFileStoreKeepsMemoryStateWhenPersistenceFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	store, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	original := Snapshot{ProviderName: "mock", SyncedAt: time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)}
	if err := store.Put(original); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0750); err != nil {
		t.Fatal(err)
	}

	replacement := Snapshot{ProviderName: "mock", SyncedAt: original.SyncedAt.Add(time.Minute)}
	if err := store.Put(replacement); err == nil {
		t.Fatal("expected persistence failure")
	}

	got, ok := store.Get("mock")
	if !ok || !got.SyncedAt.Equal(original.SyncedAt) {
		t.Fatalf("memory state changed despite persistence failure: %#v", got)
	}
}
