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
	t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", "")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_PATH", "")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_CURRENCY", "")
	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "")
	t.Setenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD", "")
	t.Setenv("DESKAPROVIDER_CATALOG_SYNC_INTERVAL", "")
	t.Setenv("DESKAPROVIDER_CATALOG_MAX_AGE", "")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_SNAPSHOT_MAX_AGE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorePath != defaultStorePath || cfg.TransactionStorePath != defaultTransactionStorePath || cfg.SyncInterval != defaultSyncInterval ||
		cfg.FailureThreshold != defaultFailureThreshold || cfg.Currency != defaultCurrency || cfg.CatalogSyncInterval != defaultCatalogSyncInterval || cfg.CatalogMaxAge != defaultCatalogMaxAge || cfg.OperationalSnapshotMaxAge != defaultOperationalSnapshotMaxAge {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "not-a-duration")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid interval error")
	}

	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "30s")
	t.Setenv("DESKAPROVIDER_CATALOG_MAX_AGE", "not-a-duration")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid catalog max age error")
	}
	t.Setenv("DESKAPROVIDER_CATALOG_MAX_AGE", "45m")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_SNAPSHOT_MAX_AGE", "not-a-duration")
	if _, err := LoadConfig(); err == nil { t.Fatal("expected invalid operational snapshot max age error") }
	t.Setenv("DESKAPROVIDER_OPERATIONAL_SNAPSHOT_MAX_AGE", "2m")
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
	providerStatePath := filepath.Join(t.TempDir(), "provider-state", "state.json")
	t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", providerStatePath)
	transactionStorePath := filepath.Join(t.TempDir(), "transactions", "state.json")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_PATH", transactionStorePath)
	t.Setenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL", "45s")
	t.Setenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD", "4")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_CURRENCY", "IDR")

	service, err := NewFromEnvironment(nil)
	if err != nil {
		t.Fatal(err)
	}
	if service.interval != 45*time.Second || service.syncService.FailureThreshold != 4 || service.PurchaseService() == nil {
		t.Fatalf("unexpected service configuration: interval=%s threshold=%d", service.interval, service.syncService.FailureThreshold)
	}
	if service.providerState == nil {
		t.Fatal("expected provider state store")
	}
	state, ok := service.providerState.Get("digiflazz")
	if !ok {
		t.Fatal("expected registered provider state")
	}
	if state.Enabled() {
		t.Fatal("provider must remain disabled until explicit administrative enablement")
	}
	if !state.Supports(operational.CapabilityPPOB) || !state.Supports(operational.CapabilityBalance) || !state.Supports(operational.CapabilityWebhook) {
		t.Fatalf("unexpected provider capabilities: %#v", state.Capabilities)
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("store should be created on first write, stat error: %v", err)
	}
}

func TestNewFromEnvironmentPreservesEnabledProviderLifecycleAcrossRestart(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH", filepath.Join(root, "operational", "snapshots.json"))
	t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", filepath.Join(root, "provider-state", "state.json"))
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_PATH", filepath.Join(root, "transactions", "state.json"))

	first, err := NewFromEnvironment(nil)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := operational.NewProviderAdminService(first.providerState)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Enable("digiflazz"); err != nil {
		t.Fatal(err)
	}

	second, err := NewFromEnvironment(nil)
	if err != nil {
		t.Fatal(err)
	}
	state, ok := second.providerState.Get("digiflazz")
	if !ok {
		t.Fatal("expected digiflazz state after restart")
	}
	if !state.Enabled() {
		t.Fatal("expected enabled lifecycle to survive runtime restart")
	}
	if !state.Supports(operational.CapabilityPPOB) || !state.Supports(operational.CapabilityBalance) || !state.Supports(operational.CapabilityWebhook) {
		t.Fatalf("unexpected capabilities after restart: %#v", state.Capabilities)
	}
}

func TestServiceRestartRecoversPersistedOperationalSnapshot(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "operational", "snapshots.json")

	firstProvider := &balanceMock{
		Provider: mock.New(mock.Config{
			Products:       []provider.Product{{Code: "xld10", Name: "Test"}},
			PurchaseStatus: provider.StatusSuccess,
		}),
		balance: 1750000,
	}
	firstRegistry := provider.NewRegistry()
	if err := firstRegistry.Register("mock", firstProvider); err != nil {
		t.Fatal(err)
	}
	firstStore, err := operational.NewJSONFileStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	firstSync, err := operational.NewSyncService(firstRegistry, firstStore, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	firstService, err := New(firstSync, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := firstSync.SyncAll(ctx); len(err) != 0 {
		t.Fatalf("initial sync failed: %#v", err)
	}
	cancel()

	secondProvider := &balanceMock{
		Provider: mock.New(mock.Config{
			Products:       []provider.Product{{Code: "xld10", Name: "Test"}},
			PurchaseStatus: provider.StatusSuccess,
		}),
		balance: 1800000,
	}
	secondRegistry := provider.NewRegistry()
	if err := secondRegistry.Register("mock", secondProvider); err != nil {
		t.Fatal(err)
	}
	secondStore, err := operational.NewJSONFileStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	recovered, ok := secondStore.Get("mock")
	if !ok {
		t.Fatal("expected persisted snapshot after restart")
	}
	if recovered.Balance != 1750000 || recovered.Health != operational.HealthHealthy {
		t.Fatalf("unexpected recovered snapshot: %#v", recovered)
	}

	secondSync, err := operational.NewSyncService(secondRegistry, secondStore, "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	secondService, err := New(secondSync, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if firstService == secondService {
		t.Fatal("expected distinct service instances across restart")
	}

	if errByProvider := secondSync.SyncAll(context.Background()); len(errByProvider) != 0 {
		t.Fatalf("recovery sync failed: %#v", errByProvider)
	}
	updated, ok := secondStore.Get("mock")
	if !ok {
		t.Fatal("expected updated snapshot after recovery sync")
	}
	if updated.Balance != 1800000 || updated.Health != operational.HealthHealthy || updated.ConsecutiveFailures != 0 {
		t.Fatalf("unexpected post-restart snapshot: %#v", updated)
	}
}
