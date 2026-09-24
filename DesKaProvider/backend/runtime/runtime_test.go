package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH", "")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_CURRENCY", "")
	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "")
	t.Setenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorePath != defaultStorePath {
		t.Fatalf("unexpected store path: %q", cfg.StorePath)
	}
	if cfg.SyncInterval != defaultSyncInterval {
		t.Fatalf("unexpected sync interval: %s", cfg.SyncInterval)
	}
	if cfg.FailureThreshold != defaultFailureThreshold {
		t.Fatalf("unexpected failure threshold: %d", cfg.FailureThreshold)
	}
	if cfg.Currency != defaultCurrency {
		t.Fatalf("unexpected currency: %q", cfg.Currency)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "not-a-duration")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid interval error")
	}

	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "30s")
	t.Setenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD", "0")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid failure threshold error")
	}
}

func TestServiceRunStopsOnContextCancellation(t *testing.T) {
	mock := Mock.New(Mock.Config{
		Products: map[string]provider.Product{"xld10": {Code: "xld10", Name: "Test"}},
		PurchaseStatus: provider.StatusSuccess,
	})
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	store := operational.NewMemoryStore()
	syncService, err := operational.NewSyncService(registry, store, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		if _, ok := store.Get("mock"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("initial synchronization did not occur")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("unexpected shutdown error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("service did not stop after cancellation")
	}
}

func TestNewFromEnvironmentBuildsDurableService(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "operational", "snapshots.json")
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH", storePath)
	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "45s")
	t.Setenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD", "4")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_CURRENCY", "IDR")

	service, err := NewFromEnvironment(nil)
	if err != nil {
		t.Fatal(err)
	}
	if service.interval != 45*time.Second {
		t.Fatalf("unexpected interval: %s", service.interval)
	}
	if service.syncService.FailureThreshold != 4 {
		t.Fatalf("unexpected threshold: %d", service.syncService.FailureThreshold)
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("store should be created on first write, stat error: %v", err)
	}
}
