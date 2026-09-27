package catalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
)

func TestJSONFileStatusPersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog-sync-status.json")
	store, err := NewJSONFileStatusPersistence(path)
	if err != nil {
		t.Fatal(err)
	}
	statuses := []SyncStatus{{
		ProviderName:        "mock",
		LastAttemptAt:       time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
		LastSuccessAt:       time.Date(2026, 9, 28, 12, 1, 0, 0, time.UTC),
		ConsecutiveFailures: 2,
		LastError:           "temporary provider error",
	}}
	if err := store.Save(statuses); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ProviderName != "mock" || !got[0].LastAttemptAt.Equal(statuses[0].LastAttemptAt) || got[0].ConsecutiveFailures != 2 || got[0].LastError != statuses[0].LastError {
		t.Fatalf("unexpected persisted status: %#v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 status store, got %o", info.Mode().Perm())
	}
}

func TestJSONFileStatusPersistenceRejectsInvalidState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog-sync-status.json")
	if err := os.WriteFile(path, []byte(`{"statuses":[{"provider_name":"mock","consecutive_failures":-1}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewJSONFileStatusPersistence(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected invalid status state to be rejected")
	}
}

func TestSyncServiceRestoresPersistedStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog-sync-status.json")
	persistence, err := NewJSONFileStatusPersistence(path)
	if err != nil {
		t.Fatal(err)
	}
	status := SyncStatus{
		ProviderName:        "mock",
		LastAttemptAt:       time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
		LastSuccessAt:       time.Date(2026, 9, 28, 11, 59, 0, 0, time.UTC),
		ConsecutiveFailures: 3,
		LastError:           "provider unavailable",
	}
	if err := persistence.Save([]SyncStatus{status}); err != nil {
		t.Fatal(err)
	}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock.New(mock.Config{})); err != nil {
		t.Fatal(err)
	}
	svc, err := NewSyncServiceWithStatusPersistence(registry, NewMemoryStore(), persistence)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := svc.Status("mock")
	if !ok {
		t.Fatal("expected persisted status after service construction")
	}
	if got != status {
		t.Fatalf("unexpected restored status: %#v", got)
	}
}
