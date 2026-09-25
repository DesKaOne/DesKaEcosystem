package operational

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestSyncWorkerLifecycleStartsAndStopsOwnedWorker(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", balanceStub{balance: 2200000}); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	svc, err := NewSyncService(registry, store, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewSyncWorkerLifecycle(svc, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := lifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Start(context.Background()); !errors.Is(err, ErrSyncWorkerRunning) {
		t.Fatalf("expected already-running error, got %v", err)
	}

	deadline := time.After(time.Second)
	for {
		if snapshot, ok := store.Get("mock"); ok {
			if snapshot.Balance != 2200000 || snapshot.Health != HealthHealthy {
				t.Fatalf("unexpected startup snapshot: %#v", snapshot)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("owned worker did not perform immediate synchronization")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lifecycle.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

func TestSyncWorkerLifecycleRecoversPersistedSnapshotAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider-operational.json")

	registry := provider.NewRegistry()
	if err := registry.Register("mock", balanceStub{balance: 2200000}); err != nil {
		t.Fatal(err)
	}
	store, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewSyncService(registry, store, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewSyncWorkerLifecycle(svc, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	if err := first.Shutdown(shutdownCtx); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()

	restartedStore, err := NewJSONFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted, ok := restartedStore.Get("mock")
	if !ok {
		t.Fatal("expected persisted snapshot after restart")
	}
	if persisted.Balance != 2200000 || persisted.Health != HealthHealthy {
		t.Fatalf("unexpected persisted snapshot: %#v", persisted)
	}

	restartedRegistry := provider.NewRegistry()
	if err := restartedRegistry.Register("mock", balanceStub{balance: 3300000}); err != nil {
		t.Fatal(err)
	}
	restartedService, err := NewSyncService(restartedRegistry, restartedStore, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSyncWorkerLifecycle(restartedService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(time.Second)
	for {
		snapshot, ok := restartedStore.Get("mock")
		if ok && snapshot.Balance == 3300000 && snapshot.Health == HealthHealthy {
			break
		}
		select {
		case <-deadline:
			t.Fatal("restarted worker did not refresh the persisted snapshot")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	shutdownCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := second.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}
