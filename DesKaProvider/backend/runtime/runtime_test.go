package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

type balanceMock struct {
	*mock.Provider
	balance int64
}

func (p *balanceMock) GetBalance(context.Context) (int64, error) {
	return p.balance, nil
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH", "")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_CURRENCY", "")
	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "")
	t.Setenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorePath != defaultStorePath || cfg.SyncInterval != defaultSyncInterval ||
		cfg.FailureThreshold != defaultFailureThreshold || cfg.Currency != defaultCurrency {
		t.Fatalf("unexpected defaults: %#v", cfg)
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
	mockProvider := &balanceMock{
		Provider: mock.New(mock.Config{
			Products:       []provider.Product{{Code: "xld10", Name: "Test"}},
			PurchaseStatus: provider.StatusSuccess,
		}),
		balance: 1500000,
	}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil {
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
		if snapshot, ok := store.Get("mock"); ok {
			if snapshot.Balance != 1500000 || snapshot.Health != operational.HealthHealthy {
				t.Fatalf("unexpected initial snapshot: %#v", snapshot)
			}
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
	if service.interval != 45*time.Second || service.syncService.FailureThreshold != 4 {
		t.Fatalf("unexpected service configuration: interval=%s threshold=%d", service.interval, service.syncService.FailureThreshold)
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("store should be created on first write, stat error: %v", err)
	}
}
