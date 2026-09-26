package operational

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONFileStorePersistsAndRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operational", "snapshots.json")
	first, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{
		ProviderName:       "digiflazz",
		Balance:            1500000,
		Currency:           "IDR",
		Health:             HealthHealthy,
		LastCheckedAt:      time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
		LastSuccessAt:      time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
		ConsecutiveFailures: 0,
	}
	if err := first.Put(snapshot); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := recovered.Get("digiflazz")
	if !ok {
		t.Fatal("expected persisted snapshot")
	}
	if got.ProviderName != snapshot.ProviderName || got.Balance != snapshot.Balance || got.Health != snapshot.Health {
		t.Fatalf("unexpected recovered snapshot: %#v", got)
	}
	if !got.LastCheckedAt.Equal(snapshot.LastCheckedAt) || !got.LastSuccessAt.Equal(snapshot.LastSuccessAt) {
		t.Fatalf("unexpected recovered timestamps: %#v", got)
	}
}

func TestJSONFileStoreRejectsCorruptState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshots.json")
	if err := writeFile(path, []byte("{invalid")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSONFileStore(path); err == nil {
		t.Fatal("expected corrupt state error")
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}


func TestJSONFileStoreDoesNotMutateMemoryWhenPersistenceFails(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := &JSONFileStore{
		path: filepath.Join(blocked, "snapshots.json"),
		snapshots: make(map[string]Snapshot),
	}
	err := store.Put(Snapshot{ProviderName: "mock", Balance: 1000, Currency: "IDR"})
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	if _, ok := store.Get("mock"); ok {
		t.Fatal("snapshot must not remain in memory after persistence failure")
	}
}

