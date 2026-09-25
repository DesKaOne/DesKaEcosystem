package runtime

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
	"strings"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
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
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StorePath != defaultStorePath || cfg.TransactionStorePath != defaultTransactionStorePath || cfg.SyncInterval != defaultSyncInterval ||
		cfg.FailureThreshold != defaultFailureThreshold || cfg.Currency != defaultCurrency || cfg.TransactionStoreDriver != defaultTransactionStoreDriver || cfg.AuditStoreDriver != defaultAuditStoreDriver || cfg.CatalogSyncInterval != defaultCatalogSyncInterval || cfg.CatalogMaxAge != defaultCatalogMaxAge || cfg.OperationalSnapshotMaxAge != defaultOperationalSnapshotMaxAge {
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

type initializationCloseErrorDB struct {
	closeErr error
	closeCount int
}

func (db *initializationCloseErrorDB) Close() error {
	db.closeCount++
	return db.closeErr
}

func TestServiceRunRejectsConcurrentReentryAtBalanceLifecycle(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 1900000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstDone := make(chan error, 1)
	go func() { firstDone <- service.Run(ctx) }()

	deadline := time.After(time.Second)
	for !service.balanceLifecycle.Running() {
		select {
		case <-deadline:
			t.Fatal("balance lifecycle did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	secondErr := service.Run(context.Background())
	if !errors.Is(secondErr, operational.ErrSyncWorkerRunning) {
		t.Fatalf("expected concurrent Run to reject duplicate worker start, got %v", secondErr)
	}

	cancel()
	select {
	case err := <-firstDone:
		if err != context.Canceled {
			t.Fatalf("unexpected first Run shutdown error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first Run did not shut down")
	}
}

func TestServiceRunRejectsConcurrentReentryWithoutClosingActiveDatabase(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 2000000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	closeDB := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(closeDB, nil)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan error, 1)
	go func() { firstDone <- service.Run(ctx) }()

	deadline := time.After(time.Second)
	for !service.balanceLifecycle.Running() {
		select {
		case <-deadline:
			t.Fatal("balance lifecycle did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	secondErr := service.Run(context.Background())
	if !errors.Is(secondErr, operational.ErrSyncWorkerRunning) {
		t.Fatalf("expected concurrent Run to reject duplicate worker start, got %v", secondErr)
	}
	if closeDB.closeCount != 0 {
		t.Fatalf("concurrent Run must not close the active runtime database, got close count %d", closeDB.closeCount)
	}

	cancel()
	select {
	case err := <-firstDone:
		if err != context.Canceled {
			t.Fatalf("unexpected first Run shutdown error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first Run did not shut down")
	}
	if closeDB.closeCount != 1 {
		t.Fatalf("expected active Run to close the database exactly once, got %d", closeDB.closeCount)
	}
}

func TestServiceCloseDoesNotCloseDatabaseWhileRunIsActive(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 2100000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	closeDB := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(closeDB, nil)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	deadline := time.After(time.Second)
	for !service.balanceLifecycle.Running() {
		select {
		case <-deadline:
			t.Fatal("balance lifecycle did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	if err := service.Close(); err == nil {
		t.Fatal("expected Close to reject active runtime ownership")
	}
	if closeDB.closeCount != 0 {
		t.Fatalf("explicit Close must not close an active runtime database, got %d", closeDB.closeCount)
	}

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("unexpected Run shutdown error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not shut down")
	}
	if closeDB.closeCount != 1 {
		t.Fatalf("expected active Run to close database exactly once, got %d", closeDB.closeCount)
	}
}

func TestServiceCloseRemainsIdempotentAfterRepeatedRunShutdown(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 1950000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 3)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	closeDB := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(closeDB, nil)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	deadline := time.After(time.Second)
	for !service.balanceLifecycle.Running() {
		select {
		case <-deadline:
			t.Fatal("balance lifecycle did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("unexpected Run shutdown error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not shut down")
	}

	for i := 0; i < 3; i++ {
		if err := service.Close(); err != nil {
			t.Fatalf("repeated service close failed on attempt %d: %v", i+1, err)
		}
	}
	if closeDB.closeCount != 1 {
		t.Fatalf("expected database close exactly once after repeated shutdown/close, got %d", closeDB.closeCount)
	}
}

func TestRuntimeInitializationCleanupErrorIsObservable(t *testing.T) {
	primary := errors.New("initialization failed")
	cleanupErr := errors.New("cleanup failed")
	transactionDB := &initializationCloseErrorDB{closeErr: cleanupErr}
	got := withRuntimeInitializationCleanupError(primary, transactionDB, nil)
	if !errors.Is(got, primary) {
		t.Fatalf("expected primary initialization error to remain discoverable: %v", got)
	}
	if !errors.Is(got, cleanupErr) {
		t.Fatalf("expected cleanup error to be observable: %v", got)
	}
	if transactionDB.closeCount != 1 {
		t.Fatalf("expected one cleanup close, got %d", transactionDB.closeCount)
	}
}

func TestRuntimeInitializationCleanupPreservesPrimaryWhenCleanupSucceeds(t *testing.T) {
	primary := errors.New("initialization failed")
	transactionDB := &initializationCloseErrorDB{}
	got := withRuntimeInitializationCleanupError(primary, transactionDB, nil)
	if got != primary {
		t.Fatalf("expected primary error identity to be preserved, got %v", got)
	}
	if transactionDB.closeCount != 1 {
		t.Fatalf("expected one cleanup close, got %d", transactionDB.closeCount)
	}
}

func TestRuntimeInitializationCleanupDoesNotDoubleCloseSharedHandle(t *testing.T) {
	cleanupErr := errors.New("cleanup failed")
	shared := &initializationCloseErrorDB{closeErr: cleanupErr}
	got := withRuntimeInitializationCleanupError(errors.New("initialization failed"), shared, shared)
	if !errors.Is(got, cleanupErr) {
		t.Fatalf("expected cleanup error to be observable: %v", got)
	}
	if shared.closeCount != 1 {
		t.Fatalf("expected shared handle to close once, got %d", shared.closeCount)
	}
}

func TestRuntimeDatabaseOwnershipSuccessPathTransfersOwnershipToService(t *testing.T) {
	transactionDB := &closeErrorDB{}
	auditDB := &closeErrorDB{}
	ownership := newRuntimeDatabaseOwnership(transactionDB, auditDB)
	ownership.transferToService()
	if err := ownership.cleanupBeforeTransfer(); err != nil { t.Fatalf("successful handoff must disable initialization cleanup: %v", err) }
	if transactionDB.closeCount != 0 || auditDB.closeCount != 0 { t.Fatalf("initialization guard closed transferred resources: tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount) }
	if err := ownership.closeOwned(); err != nil { t.Fatalf("service shutdown close failed: %v", err) }
	if transactionDB.closeCount != 1 || auditDB.closeCount != 1 { t.Fatalf("expected service to close each resource once: tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount) }
	if err := ownership.closeOwned(); err != nil { t.Fatalf("repeated service shutdown should return the recorded close result: %v", err) }
	if transactionDB.closeCount != 1 || auditDB.closeCount != 1 { t.Fatalf("repeated shutdown double-closed resources: tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount) }
}

func TestRuntimeDatabaseOwnershipSuccessPathSharedResourceClosesOnce(t *testing.T) {
	shared := &closeErrorDB{}
	ownership := newRuntimeDatabaseOwnership(shared, shared)
	ownership.transferToService()
	if err := ownership.cleanupBeforeTransfer(); err != nil { t.Fatalf("unexpected guard cleanup error: %v", err) }
	if shared.closeCount != 0 { t.Fatalf("shared resource closed before shutdown: %d", shared.closeCount) }
	if err := ownership.closeOwned(); err != nil { t.Fatalf("shutdown close failed: %v", err) }
	if shared.closeCount != 1 { t.Fatalf("expected shared resource to close once, got %d", shared.closeCount) }
	if err := ownership.closeOwned(); err != nil { t.Fatalf("second shutdown close returned unexpected error: %v", err) }
	if shared.closeCount != 1 { t.Fatalf("repeated shutdown double-closed shared resource: %d", shared.closeCount) }
}

func TestRuntimeDatabaseOwnershipSuccessPathDedicatedAuditResource(t *testing.T) {
	auditDB := &closeErrorDB{}
	ownership := newRuntimeDatabaseOwnership(nil, auditDB)
	ownership.transferToService()
	if err := ownership.cleanupBeforeTransfer(); err != nil { t.Fatalf("unexpected guard cleanup error: %v", err) }
	if auditDB.closeCount != 0 { t.Fatalf("dedicated audit resource closed before shutdown: %d", auditDB.closeCount) }
	if err := ownership.closeOwned(); err != nil { t.Fatalf("shutdown close failed: %v", err) }
	if auditDB.closeCount != 1 { t.Fatalf("expected dedicated audit resource to close once, got %d", auditDB.closeCount) }
}

func TestRuntimeShutdownContextPreservesParentDeadline(t *testing.T) {
	deadline := time.Now().Add(80 * time.Millisecond)
	parent, parentCancel := context.WithDeadline(context.Background(), deadline)
	defer parentCancel()

	shutdownCtx, cancel := runtimeShutdownContext(parent)
	defer cancel()

	gotDeadline, ok := shutdownCtx.Deadline()
	if !ok {
		t.Fatal("expected shutdown context to preserve the parent deadline")
	}
	if gotDeadline.Before(deadline.Add(-5 * time.Millisecond)) || gotDeadline.After(deadline.Add(5 * time.Millisecond)) {
		t.Fatalf("unexpected shutdown deadline: got %v want about %v", gotDeadline, deadline)
	}
	select {
	case <-shutdownCtx.Done():
		t.Fatal("shutdown context canceled before parent deadline")
	default:
	}
}

func TestRuntimeShutdownContextDoesNotInheritCancellation(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel()

	shutdownCtx, cancel := runtimeShutdownContext(parent)
	defer cancel()

	select {
	case <-shutdownCtx.Done():
		t.Fatal("shutdown context must not inherit already-canceled parent state")
	default:
	}
}

func TestServiceRunCanRestartAfterCompletedShutdown(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 2150000}
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

	runOnce := func() {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- service.Run(ctx) }()

		deadline := time.After(time.Second)
		for !service.balanceLifecycle.Running() {
			select {
			case <-deadline:
				t.Fatal("balance lifecycle did not start")
			default:
				time.Sleep(time.Millisecond)
			}
		}
		cancel()
		select {
		case err := <-done:
			if err != context.Canceled {
				t.Fatalf("unexpected Run shutdown error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Run did not shut down")
		}
		if service.balanceLifecycle.Running() {
			t.Fatal("expected balance lifecycle to be stopped after Run shutdown")
		}
	}

	runOnce()
	runOnce()
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
	if err := registry.Register("mock", mockProvider); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	syncService, err := operational.NewSyncService(registry, store, "IDR", 3)
	if err != nil { t.Fatal(err) }
	service, err := New(syncService, time.Hour)
	if err != nil { t.Fatal(err) }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		if snapshot, ok := store.Get("mock"); ok {
			if snapshot.Balance != 1500000 || snapshot.Health != operational.HealthHealthy { t.Fatalf("unexpected initial snapshot: %#v", snapshot) }
			break
		}
		select {
		case <-deadline: t.Fatal("initial synchronization did not occur")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled { t.Fatalf("unexpected shutdown error: %v", err) }
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
	if err != nil { t.Fatal(err) }
	if service.interval != 45*time.Second || service.syncService.FailureThreshold != 4 || service.PurchaseService() == nil { t.Fatalf("unexpected service configuration: interval=%s threshold=%d", service.interval, service.syncService.FailureThreshold) }
	if service.providerState == nil { t.Fatal("expected provider state store") }
	state, ok := service.providerState.Get("digiflazz")
	if !ok { t.Fatal("expected registered provider state") }
	if state.Enabled() { t.Fatal("provider must remain disabled until explicit administrative enablement") }
	if !state.Supports(operational.CapabilityPPOB) || !state.Supports(operational.CapabilityBalance) || !state.Supports(operational.CapabilityWebhook) { t.Fatalf("unexpected provider capabilities: %#v", state.Capabilities) }
	if _, err := os.Stat(storePath); !os.IsNotExist(err) { t.Fatalf("store should be created on first write, stat error: %v", err) }
}

func TestNewFromEnvironmentPreservesEnabledProviderLifecycleAcrossRestart(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH", filepath.Join(root, "operational", "snapshots.json"))
	t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", filepath.Join(root, "provider-state", "state.json"))
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_PATH", filepath.Join(root, "transactions", "state.json"))

	first, err := NewFromEnvironment(nil)
	if err != nil { t.Fatal(err) }
	admin, err := operational.NewProviderAdminService(first.providerState)
	if err != nil { t.Fatal(err) }
	if _, err := admin.Enable("digiflazz"); err != nil { t.Fatal(err) }

	second, err := NewFromEnvironment(nil)
	if err != nil { t.Fatal(err) }
	state, ok := second.providerState.Get("digiflazz")
	if !ok { t.Fatal("expected digiflazz state after restart") }
	if !state.Enabled() { t.Fatal("expected enabled lifecycle to survive runtime restart") }
	if !state.Supports(operational.CapabilityPPOB) || !state.Supports(operational.CapabilityBalance) || !state.Supports(operational.CapabilityWebhook) { t.Fatalf("unexpected capabilities after restart: %#v", state.Capabilities) }
}

func TestServiceRestartRecoversPersistedOperationalSnapshot(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "operational", "snapshots.json")
	firstProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}, PurchaseStatus: provider.StatusSuccess}), balance: 1750000}
	firstRegistry := provider.NewRegistry()
	if err := firstRegistry.Register("mock", firstProvider); err != nil { t.Fatal(err) }
	firstStore, err := operational.NewJSONFileStore(storePath)
	if err != nil { t.Fatal(err) }
	firstSync, err := operational.NewSyncService(firstRegistry, firstStore, "IDR", 3)
	if err != nil { t.Fatal(err) }
	firstService, err := New(firstSync, time.Hour)
	if err != nil { t.Fatal(err) }
	ctx, cancel := context.WithCancel(context.Background())
	if err := firstSync.SyncAll(ctx); len(err) != 0 { t.Fatalf("initial sync failed: %#v", err) }
	cancel()
	secondProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}, PurchaseStatus: provider.StatusSuccess}), balance: 1800000}
	secondRegistry := provider.NewRegistry()
	if err := secondRegistry.Register("mock", secondProvider); err != nil { t.Fatal(err) }
	secondStore, err := operational.NewJSONFileStore(storePath)
	if err != nil { t.Fatal(err) }
	recovered, ok := secondStore.Get("mock")
	if !ok { t.Fatal("expected persisted snapshot after restart") }
	if recovered.Balance != 1750000 || recovered.Health != operational.HealthHealthy { t.Fatalf("unexpected recovered snapshot: %#v", recovered) }
	secondSync, err := operational.NewSyncService(secondRegistry, secondStore, "IDR", 3)
	if err != nil { t.Fatal(err) }
	secondService, err := New(secondSync, time.Hour)
	if err != nil { t.Fatal(err) }
	if firstService == secondService { t.Fatal("expected distinct service instances across restart") }
	if errByProvider := secondSync.SyncAll(context.Background()); len(errByProvider) != 0 { t.Fatalf("recovery sync failed: %#v", errByProvider) }
	updated, ok := secondStore.Get("mock")
	if !ok { t.Fatal("expected updated snapshot after recovery sync") }
	if updated.Balance != 1800000 || updated.Health != operational.HealthHealthy || updated.ConsecutiveFailures != 0 { t.Fatalf("unexpected post-restart snapshot: %#v", updated) }
}

func TestNewFromEnvironmentContextRejectsCanceledInitialization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewFromEnvironmentContext(ctx, nil); err != context.Canceled { t.Fatalf("expected context.Canceled, got %v", err) }
}

func TestNewFromEnvironmentContextRequiresContext(t *testing.T) {
	if _, err := NewFromEnvironmentContext(nil, nil); err == nil { t.Fatal("expected initialization context error") }
}

func TestRuntimeInitializationContextCheckpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := checkRuntimeInitializationContext(ctx); err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if err := checkRuntimeInitializationContext(context.Background()); err != nil {
		t.Fatalf("expected active context to pass checkpoint, got %v", err)
	}
}

func TestNewFromEnvironmentContextRejectsProviderStateInitializationFailureAfterOwnershipSetup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH", filepath.Join(root, "operational", "snapshots.json"))
	providerStatePath := filepath.Join(root, "provider-state", "state.json")
	if err := os.MkdirAll(filepath.Dir(providerStatePath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(providerStatePath, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", providerStatePath)
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_PATH", filepath.Join(root, "transactions", "state.json"))
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "json")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "memory")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")

	service, err := NewFromEnvironmentContext(context.Background(), nil)
	if err == nil {
		t.Fatal("expected provider state initialization failure")
	}
	if service != nil {
		t.Fatal("expected failed initialization not to return a service")
	}
	if !strings.Contains(err.Error(), "decode provider state store") {
		t.Fatalf("expected provider state decode error, got %v", err)
	}
}

func TestLoadConfigPostgresRequiresDSN(t *testing.T) {
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")
	if _, err := LoadConfig(); err == nil { t.Fatal("expected PostgreSQL DSN requirement") }
}

func TestLoadConfigRejectsUnknownTransactionStoreDriver(t *testing.T) {
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "sqlite")
	if _, err := LoadConfig(); err == nil { t.Fatal("expected unknown transaction store driver error") }
}

func TestLoadConfigAcceptsPostgresTransactionStore(t *testing.T) {
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "postgres://test")
	cfg, err := LoadConfig()
	if err != nil { t.Fatal(err) }
	if cfg.TransactionStoreDriver != "postgres" || cfg.PostgresDSN != "postgres://test" { t.Fatalf("unexpected PostgreSQL config: %#v", cfg) }
}

func TestOpenTransactionStoreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: "postgres://invalid"}
	if _, _, err := openTransactionStore(ctx, cfg); err != context.Canceled { t.Fatalf("expected context.Canceled, got %v", err) }
}

func TestLoadConfigPostgresAuditRequiresDSN(t *testing.T) {
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")
	if _, err := LoadConfig(); err == nil { t.Fatal("expected PostgreSQL DSN requirement for audit store") }
}

func TestLoadConfigAcceptsPostgresAuditStore(t *testing.T) {
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "postgres://test")
	cfg, err := LoadConfig()
	if err != nil { t.Fatal(err) }
	if cfg.AuditStoreDriver != "postgres" || cfg.PostgresDSN != "postgres://test" { t.Fatalf("unexpected PostgreSQL audit config: %#v", cfg) }
}

func TestLoadConfigRejectsUnknownAuditStoreDriver(t *testing.T) {
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "sqlite")
	if _, err := LoadConfig(); err == nil { t.Fatal("expected unknown audit store driver error") }
}

type closeErrorDB struct {
	err error
	closed bool
	closeCount int
}

func (d *closeErrorDB) Close() error {
	d.closed = true
	d.closeCount++
	return d.err
}

func TestCloseRuntimeDatabasesIgnoresTypedNilHandles(t *testing.T) {
	var transactionDB *sql.DB
	var auditDB *sql.DB
	if err := closeRuntimeDatabases(transactionDB, auditDB); err != nil {
		t.Fatalf("typed-nil database handles must be ignored: %v", err)
	}
}

func TestCloseRuntimeDatabasesPropagatesCloseErrors(t *testing.T) {
	transactionErr := errors.New("transaction close failed")
	auditErr := errors.New("audit close failed")
	transactionDB := &closeErrorDB{err: transactionErr}
	auditDB := &closeErrorDB{err: auditErr}
	err := closeRuntimeDatabases(transactionDB, auditDB)
	if !errors.Is(err, transactionErr) || !errors.Is(err, auditErr) { t.Fatalf("expected both close errors, got %v", err) }
	if !transactionDB.closed || !auditDB.closed { t.Fatal("expected both database handles to be closed") }
}

func TestCloseRuntimeDatabasesDoesNotDoubleCloseSharedHandle(t *testing.T) {
	transactionDB := &closeErrorDB{}
	err := closeRuntimeDatabases(transactionDB, transactionDB)
	if err != nil { t.Fatalf("unexpected close error: %v", err) }
	if !transactionDB.closed { t.Fatal("expected shared database handle to be closed") }
}

func TestServiceCloseIsIdempotent(t *testing.T) {
	transactionDB := &closeErrorDB{}
	auditDB := &closeErrorDB{}
	service := &Service{databaseOwnership: newRuntimeDatabaseOwnership(transactionDB, auditDB)}
	service.databaseOwnership.transferToService()
	if err := service.Close(); err != nil { t.Fatalf("first close failed: %v", err) }
	if err := service.Close(); err != nil { t.Fatalf("second close failed: %v", err) }
	if transactionDB.closeCount != 1 || auditDB.closeCount != 1 { t.Fatalf("expected Close to release each resource once: tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount) }
}

func TestServiceClosePreservesCloseErrorAcrossRepeatedCalls(t *testing.T) {
	closeErr := errors.New("database close failed")
	transactionDB := &closeErrorDB{err: closeErr}
	service := &Service{databaseOwnership: newRuntimeDatabaseOwnership(transactionDB, nil)}
	service.databaseOwnership.transferToService()
	first := service.Close()
	second := service.Close()
	if !errors.Is(first, closeErr) || !errors.Is(second, closeErr) { t.Fatalf("expected close error to remain discoverable: first=%v second=%v", first, second) }
	if transactionDB.closeCount != 1 { t.Fatalf("expected one underlying close, got %d", transactionDB.closeCount) }
}

func TestServiceRunUsesOwnedBalanceWorkerLifecycle(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 1600000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	syncService, err := operational.NewSyncService(registry, store, "IDR", 3)
	if err != nil { t.Fatal(err) }
	service, err := New(syncService, time.Hour)
	if err != nil { t.Fatal(err) }
	if service.balanceLifecycle == nil { t.Fatal("expected runtime service to own balance worker lifecycle") }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func(){ done <- service.Run(ctx) }()
	deadline := time.After(time.Second)
	for {
		if snapshot, ok := store.Get("mock"); ok {
			if snapshot.Balance != 1600000 || snapshot.Health != operational.HealthHealthy { t.Fatalf("unexpected startup snapshot: %#v", snapshot) }
			break
		}
		select { case <-deadline: t.Fatal("runtime balance worker did not start"); default: time.Sleep(time.Millisecond) }
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled { t.Fatalf("unexpected shutdown error: %v", err) }
	case <-time.After(time.Second): t.Fatal("runtime service did not shut down")
	}
	if err := service.balanceLifecycle.Shutdown(context.Background()); err != nil { t.Fatalf("repeated lifecycle shutdown failed: %v", err) }
}

func TestServiceRunPropagatesDatabaseCloseError(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}, PurchaseStatus: provider.StatusSuccess}), balance: 1500000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil { t.Fatal(err) }
	store := operational.NewMemoryStore()
	syncService, err := operational.NewSyncService(registry, store, "IDR", 3)
	if err != nil { t.Fatal(err) }
	closeErr := errors.New("shutdown database close failed")
	service, err := New(syncService, time.Hour)
	if err != nil { t.Fatal(err) }
	service.databaseOwnership = newRuntimeDatabaseOwnership(&closeErrorDB{err: closeErr}, nil)
	service.databaseOwnership.transferToService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = service.Run(ctx)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, closeErr) { t.Fatalf("expected context cancellation and close error, got %v", err) }
}

func TestCatalogWorkerLifecycleStartAndShutdownAreDeterministic(t *testing.T) {
	lifecycle := newCatalogWorkerLifecycle()
	ctx, err := lifecycle.Start(context.Background())
	if err != nil { t.Fatal(err) }
	if ctx == nil { t.Fatal("expected derived catalog context") }
	if _, err := lifecycle.Start(context.Background()); err == nil { t.Fatal("expected duplicate catalog worker start to fail") }
	lifecycle.Shutdown(); lifecycle.Shutdown()
	ctx2, err := lifecycle.Start(context.Background())
	if err != nil { t.Fatal(err) }
	select { case <-ctx2.Done(): t.Fatal("new catalog lifecycle context canceled before shutdown"); default: }
	lifecycle.Shutdown()
	select { case <-ctx2.Done(): default: t.Fatal("expected shutdown to cancel catalog lifecycle context") }
}

func TestServiceRunUsesOwnedCatalogWorkerLifecycle(t *testing.T) {
	mockProvider := mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}})
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil { t.Fatal(err) }
	catalogStore := catalog.NewMemoryStore()
	catalogSync, err := catalog.NewSyncService(registry, catalogStore)
	if err != nil { t.Fatal(err) }
	operationalStore := operational.NewMemoryStore()
	syncService, err := operational.NewSyncService(registry, operationalStore, "IDR", 3)
	if err != nil { t.Fatal(err) }
	service, err := New(syncService, time.Hour)
	if err != nil { t.Fatal(err) }
	service.catalogSync = catalogSync
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func(){ done <- service.Run(ctx) }()
	deadline := time.After(time.Second)
	for {
		if snapshot, ok := catalogStore.Get("mock"); ok {
			if len(snapshot.Products) != 1 || snapshot.Products[0].Code != "xld10" { t.Fatalf("unexpected catalog snapshot: %#v", snapshot) }
			break
		}
		select { case <-deadline: t.Fatal("runtime catalog worker did not perform initial sync"); default: time.Sleep(time.Millisecond) }
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled { t.Fatalf("unexpected shutdown error: %v", err) }
	case <-time.After(time.Second): t.Fatal("runtime catalog lifecycle did not shut down")
	}
	select { case <-ctx.Done(): default: t.Fatal("expected service context to be canceled") }
}

func TestServiceRollbackStartedLifecyclesBeforeDatabaseClose(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 1800000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil { t.Fatal(err) }
	operationalStore := operational.NewMemoryStore()
	syncService, err := operational.NewSyncService(registry, operationalStore, "IDR", 3)
	if err != nil { t.Fatal(err) }

	service, err := New(syncService, time.Hour)
	if err != nil { t.Fatal(err) }
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogSync = &catalog.SyncService{}
	service.catalogInterval = time.Hour
	catalogStartErr := errors.New("injected catalog lifecycle start failure")
	service.catalogStart = func(context.Context) (context.Context, error) {
		return nil, catalogStartErr
	}

	db := &rollbackOrderDB{service: service}
	service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
	service.databaseOwnership.transferToService()

	if err := service.Run(context.Background()); !errors.Is(err, catalogStartErr) {
		t.Fatalf("expected injected catalog start failure, got %v", err)
	}
	if !db.closed {
		t.Fatal("expected database to be closed during rollback")
	}
	if db.workerWasRunningAtClose {
		t.Fatal("expected started balance lifecycle to be stopped before database close")
	}
	if err := service.balanceLifecycle.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected rollback to stop balance lifecycle cleanly, got %v", err)
	}
}

type rollbackOrderDB struct {
	service *Service
	closed bool
	workerWasRunningAtClose bool
}

func (db *rollbackOrderDB) Close() error {
	if db.service != nil && db.service.balanceLifecycle != nil {
		db.workerWasRunningAtClose = db.service.balanceLifecycle.Shutdown(context.Background()) != nil
	}
	db.closed = true
	return nil
}

func TestServiceRunWithBalanceAndCatalogLifecyclesClosesDeterministically(t *testing.T) {
	mockProvider := &balanceMock{Provider: mock.New(mock.Config{Products: []provider.Product{{Code: "xld10", Name: "Test"}}}), balance: 1700000}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil { t.Fatal(err) }
	operationalStore := operational.NewMemoryStore()
	syncService, err := operational.NewSyncService(registry, operationalStore, "IDR", 3)
	if err != nil { t.Fatal(err) }
	catalogStore := catalog.NewMemoryStore()
	catalogSync, err := catalog.NewSyncService(registry, catalogStore)
	if err != nil { t.Fatal(err) }
	service, err := New(syncService, time.Hour)
	if err != nil { t.Fatal(err) }
	service.catalogSync = catalogSync
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour
	closeDB := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(closeDB, nil)
	service.databaseOwnership.transferToService()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	deadline := time.After(time.Second)
	for {
		if _, ok := catalogStore.Get("mock"); ok { break }
		select { case <-deadline: t.Fatal("catalog worker did not start"); default: time.Sleep(time.Millisecond) }
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled { t.Fatalf("unexpected shutdown error: %v", err) }
	case <-time.After(time.Second): t.Fatal("service did not shut down")
	}
	if closeDB.closeCount != 1 { t.Fatalf("expected database close exactly once, got %d", closeDB.closeCount) }
	if err := service.Close(); err != nil { t.Fatalf("repeated service close failed: %v", err) }
	if closeDB.closeCount != 1 { t.Fatalf("expected repeated service close to remain single-shot, got %d", closeDB.closeCount) }
}
