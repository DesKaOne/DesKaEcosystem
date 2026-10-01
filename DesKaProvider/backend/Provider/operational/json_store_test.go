package operational

import (
	"errors"
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


func TestJSONFileStoreAmbiguousPersistenceFailsClosedInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshots.json")
	store, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	initial := Snapshot{
		ProviderName: "mock",
		Balance: 1000,
		Currency: "IDR",
		Health: HealthHealthy,
		LastCheckedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		LastSuccessAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := store.Put(initial); err != nil {
		t.Fatal(err)
	}
	store.persistHook = func(stage operationalStorePersistStage) error {
		if stage == operationalStoreAfterReplace {
			return errors.New("directory durability uncertain")
		}
		return nil
	}
	requested := initial
	requested.Balance = 500
	requested.Health = HealthUnhealthy
	requested.LastError = "provider unavailable"
	requested.LastCheckedAt = initial.LastCheckedAt.Add(time.Minute)
	requested.LastSuccessAt = requested.LastCheckedAt
	requested.ConsecutiveFailures = 3
	err = store.Put(requested)
	if !errors.Is(err, ErrOperationalPersistenceAmbiguous) {
		t.Fatalf("expected ambiguous persistence error, got %v", err)
	}
	got, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected conservative snapshot to remain in memory")
	}
	if got.Health != HealthUnhealthy || got.Balance != 500 {
		t.Fatalf("ambiguous restrictive write must fail closed in memory: %#v", got)
	}
	if !got.LastCheckedAt.Equal(initial.LastCheckedAt) {
		t.Fatalf("ambiguous write must not promote freshness: got %v want %v", got.LastCheckedAt, initial.LastCheckedAt)
	}

	recovered, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	durable, ok := recovered.Get("mock")
	if !ok || durable.Health != HealthUnhealthy || durable.Balance != 500 {
		t.Fatalf("rename-completed ambiguous write should remain recoverable from durable state: %#v", durable)
	}
}

func TestJSONFileStoreAmbiguousFirstWriteFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshots.json")
	store, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.persistHook = func(stage operationalStorePersistStage) error {
		if stage == operationalStoreAfterReplace {
			return errors.New("directory durability uncertain")
		}
		return nil
	}
	requested := Snapshot{
		ProviderName: "new-provider",
		Balance: 1000,
		Currency: "IDR",
		Health: HealthHealthy,
		LastCheckedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		LastSuccessAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := store.Put(requested); !errors.Is(err, ErrOperationalPersistenceAmbiguous) {
		t.Fatalf("expected ambiguous first-write error, got %v", err)
	}
	got, ok := store.Get("new-provider")
	if !ok {
		t.Fatal("expected fail-closed in-memory snapshot")
	}
	if got.Health != HealthUnknown || got.Balance != 0 || !got.LastCheckedAt.Equal(requested.LastCheckedAt) {
		t.Fatalf("ambiguous first write must fail closed: %#v", got)
	}
	if got.LastSuccessAt != (time.Time{}) || got.ConsecutiveFailures != 0 {
		t.Fatalf("ambiguous first write must not retain success evidence: %#v", got)
	}
}

func TestJSONFileStoreAmbiguousPermissiveWriteDoesNotPromoteMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshots.json")
	store, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	initial := Snapshot{
		ProviderName: "mock",
		Balance: 500,
		Currency: "IDR",
		Health: HealthUnhealthy,
		LastError: "provider unavailable",
		LastCheckedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		LastSuccessAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		ConsecutiveFailures: 3,
	}
	if err := store.Put(initial); err != nil {
		t.Fatal(err)
	}
	store.persistHook = func(stage operationalStorePersistStage) error {
		if stage == operationalStoreAfterReplace {
			return errors.New("directory durability uncertain")
		}
		return nil
	}
	requested := initial
	requested.Balance = 1000
	requested.Health = HealthHealthy
	requested.LastError = ""
	requested.LastCheckedAt = initial.LastCheckedAt.Add(time.Minute)
	requested.LastSuccessAt = requested.LastCheckedAt
	requested.ConsecutiveFailures = 0
	err = store.Put(requested)
	if !errors.Is(err, ErrOperationalPersistenceAmbiguous) {
		t.Fatalf("expected ambiguous persistence error, got %v", err)
	}
	got, ok := store.Get("mock")
	if !ok {
		t.Fatal("expected conservative snapshot to remain in memory")
	}
	if got.Health != HealthUnhealthy || got.Balance != 500 || !got.LastCheckedAt.Equal(initial.LastCheckedAt) || got.ConsecutiveFailures != 3 {
		t.Fatalf("ambiguous permissive write must not promote memory: %#v", got)
	}
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



func TestJSONFileStoreRejectsProviderIdentityMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshots.json")
	data := []byte(`{"snapshots":{"digiflazz":{"ProviderName":"different-provider","Balance":1000,"Currency":"IDR","Health":"healthy","LastCheckedAt":"2026-10-02T10:00:00Z","LastSuccessAt":"2026-10-02T10:00:00Z","LastError":"","ConsecutiveFailures":0}}}`)
	if err := writeFile(path, data); err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSONFileStore(path); err == nil {
		t.Fatal("expected provider identity mismatch to be rejected")
	}
}
