package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
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

func TestNewFromEnvironmentRestartSeparatesPersistedStateFromEphemeralRuntimeState(t *testing.T) {
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
	if first.balanceLifecycle.Running() {
		t.Fatal("persisted provider lifecycle state must not imply an active balance worker")
	}
	if first.databaseOwnership == nil || !first.databaseOwnership.transferred() {
		t.Fatal("expected first runtime instance to own transferred database resources")
	}
	if first.databaseOwnership.closed {
		t.Fatal("first runtime ownership must remain open before shutdown")
	}

	second, err := NewFromEnvironment(nil)
	if err != nil {
		t.Fatal(err)
	}
	state, ok := second.providerState.Get("digiflazz")
	if !ok || !state.Enabled() {
		t.Fatalf("expected persisted provider lifecycle state to survive restart: %#v", state)
	}
	if second.balanceLifecycle.Running() {
		t.Fatal("restart must not restore an active balance worker from persisted provider state")
	}
	if second.databaseOwnership == nil || !second.databaseOwnership.transferred() {
		t.Fatal("expected second runtime instance to own newly transferred database resources")
	}
	if second.databaseOwnership.closed {
		t.Fatal("second runtime ownership must start open after successful initialization")
	}
	if second.databaseOwnership == first.databaseOwnership {
		t.Fatal("persisted provider state must not reuse prior runtime database ownership")
	}
	if second.balanceLifecycle == first.balanceLifecycle {
		t.Fatal("persisted provider state must not reuse prior runtime lifecycle")
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if !first.databaseOwnership.closed {
		t.Fatal("first runtime ownership should close after explicit shutdown")
	}
	if second.databaseOwnership.closed {
		t.Fatal("closing the first runtime instance must not close the second runtime ownership")
	}

	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewFromEnvironmentCreatesFreshRuntimeOwnershipPerInstance(t *testing.T) {
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
	second, err := NewFromEnvironment(nil)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("expected distinct service instances")
	}
	if first.databaseOwnership == nil || second.databaseOwnership == nil {
		t.Fatal("expected database ownership on both runtime instances")
	}
	if first.databaseOwnership == second.databaseOwnership {
		t.Fatal("expected fresh database ownership per runtime instance")
	}
	if first.balanceLifecycle == nil || second.balanceLifecycle == nil {
		t.Fatal("expected balance lifecycle on both runtime instances")
	}
	if first.balanceLifecycle == second.balanceLifecycle {
		t.Fatal("expected fresh balance lifecycle per runtime instance")
	}
	if first.catalogLifecycle == nil || second.catalogLifecycle == nil {
		t.Fatal("expected catalog lifecycle on both runtime instances")
	}
	if first.catalogLifecycle == second.catalogLifecycle {
		t.Fatal("expected fresh catalog lifecycle per runtime instance")
	}
	if first.providerState == nil || second.providerState == nil {
		t.Fatal("expected provider state store on both runtime instances")
	}
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

func TestLoadConfigPostgresDSNRequirementMatrix(t *testing.T) {
	tests := []struct {
		name string
		transactionDriver string
		auditDriver string
		want string
	}{
		{name: "transaction only", transactionDriver: "postgres", auditDriver: "memory", want: "DESKAPROVIDER_TRANSACTION_STORE_DRIVER=postgres"},
		{name: "audit only", transactionDriver: "json", auditDriver: "postgres", want: "DESKAPROVIDER_AUDIT_STORE_DRIVER=postgres"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", tc.transactionDriver)
			t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", tc.auditDriver)
			t.Setenv("DESKAPROVIDER_POSTGRES_DSN", "")
			_, err := LoadConfig()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected DSN validation to identify %s, got %v", tc.want, err)
			}
		})
	}
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

type orderedCloseDB struct {
	name string
	order *[]string
	err error
	closeCount int
}

func (d *orderedCloseDB) Close() error {
	d.closeCount++
	*d.order = append(*d.order, d.name)
	return d.err
}

func TestCloseRuntimeDatabasesClosesTransactionBeforeAuditAndContinuesAfterError(t *testing.T) {
	order := []string{}
	transactionErr := errors.New("transaction close failed")
	transactionDB := &orderedCloseDB{name: "transaction", order: &order, err: transactionErr}
	auditDB := &orderedCloseDB{name: "audit", order: &order}
	err := closeRuntimeDatabases(transactionDB, auditDB)
	if !errors.Is(err, transactionErr) { t.Fatalf("expected transaction cleanup error, got %v", err) }
	if transactionDB.closeCount != 1 || auditDB.closeCount != 1 { t.Fatalf("expected both resources closed once: tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount) }
	if len(order) != 2 || order[0] != "transaction" || order[1] != "audit" { t.Fatalf("unexpected cleanup order: %v", order) }
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
	service.catalogSync = &catalog.SyncService{}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
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
		db.workerWasRunningAtClose = db.service.balanceLifecycle.Running()
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


func TestOpenAuditStoreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := Config{AuditStoreDriver: "postgres", PostgresDSN: "postgres://invalid"}
	if _, _, err := openAuditStore(ctx, cfg, nil); err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRuntimeInitializationCleanupWithNilPrimaryClosesAcquiredResources(t *testing.T) {
	transactionDB := &closeErrorDB{}
	auditDB := &closeErrorDB{}
	if err := withRuntimeInitializationCleanupError(nil, transactionDB, auditDB); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}
	if transactionDB.closeCount != 1 || auditDB.closeCount != 1 {
		t.Fatalf("expected each acquired resource to close once, got tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount)
	}
}

func TestRuntimeInitializationCleanupPreservesWrappedPrimaryAndCleanupIdentity(t *testing.T) {
	primary := errors.New("postgres audit initialization failed")
	cleanupErr := errors.New("postgres transaction cleanup failed")
	transactionDB := &initializationCloseErrorDB{closeErr: cleanupErr}
	wrappedPrimary := fmt.Errorf("open audit store: %w", primary)
	got := withRuntimeInitializationCleanupError(wrappedPrimary, transactionDB, nil)
	if !errors.Is(got, primary) {
		t.Fatalf("expected wrapped primary error identity to remain discoverable: %v", got)
	}
	if !errors.Is(got, cleanupErr) {
		t.Fatalf("expected cleanup error identity to remain discoverable: %v", got)
	}
	if transactionDB.closeCount != 1 {
		t.Fatalf("expected one cleanup close, got %d", transactionDB.closeCount)
	}
}




func TestServiceCloseRejectsRunningCatalogLifecycleBeforeDatabaseCleanup(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	if _, err := service.catalogLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	db := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
	service.databaseOwnership.transferToService()

	err = service.Close()
	if err == nil || err.Error() != "service close requires catalog worker shutdown" {
		t.Fatalf("expected catalog worker shutdown guard, got %v", err)
	}
	if db.closeCount != 0 {
		t.Fatalf("expected database to remain open while catalog worker is running, got %d closes", db.closeCount)
	}

	service.catalogLifecycle.Shutdown()
	if err := service.Close(); err != nil {
		t.Fatalf("expected close after catalog shutdown to succeed, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected one database close after lifecycle completion, got %d", db.closeCount)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("expected repeated close to remain idempotent, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected repeated close not to double-close database, got %d", db.closeCount)
	}
}

func TestServiceRunShutdownCompletionOrderingAndRepeatedClose(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	catalogStore := catalog.NewMemoryStore()
	catalogSync, err := catalog.NewSyncService(registry, catalogStore)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync = catalogSync
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	order := make([]string, 0, 3)
	var mu sync.Mutex
	appendOrder := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, name)
	}
	service.balanceShutdown = func(ctx context.Context) error {
		if err := service.balanceLifecycle.Shutdown(ctx); err != nil {
			return err
		}
		appendOrder("balance")
		return nil
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		appendOrder("catalog")
		return nil
	}
	tx := &orderedCloseDB{name: "transaction", order: &order}
	audit := &orderedCloseDB{name: "audit", order: &order}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}

	if service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to be stopped")
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to be stopped")
	}
	if err := service.Close(); err != nil {
		t.Fatalf("repeated service close failed: %v", err)
	}

	want := []string{"balance", "catalog", "transaction", "audit"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected shutdown completion order: got %v want %v", order, want)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected one database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}


func TestServiceCatalogStartFailurePreservesCompletionOrderingAndAllErrorIdentity(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()

	catalogStartErr := errors.New("catalog start failed")
	balanceErr := errors.New("balance rollback failed")
	transactionErr := errors.New("transaction close failed")
	auditErr := errors.New("audit close failed")

	order := make([]string, 0, 4)
	var mu sync.Mutex
	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, name)
	}

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		record("balance")
		return balanceErr
	}
	service.catalogStart = func(context.Context) (context.Context, error) {
		return nil, catalogStartErr
	}
	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := service.Run(ctx)

	for _, want := range []error{catalogStartErr, balanceErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected shutdown error identity %v, got %v", want, runErr)
		}
	}
	if !reflect.DeepEqual(order, []string{"balance", "transaction", "audit"}) {
		t.Fatalf("unexpected catalog-start failure shutdown ordering: got %v", order)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to be stopped")
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to be stopped")
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected single database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	repeatedCloseErr := service.Close()
	if !errors.Is(repeatedCloseErr, transactionErr) || !errors.Is(repeatedCloseErr, auditErr) {
		t.Fatalf("expected repeated Close to preserve database cleanup errors, got %v", repeatedCloseErr)
	}
	if errors.Is(repeatedCloseErr, balanceErr) {
		t.Fatalf("repeated Close must not replay lifecycle completion errors: %v", repeatedCloseErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}

func TestServiceRollbackDoesNotCompleteCatalogBeforeSuccessfulStart(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	balanceLifecycle, err := operational.NewSyncWorkerLifecycle(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.balanceLifecycle = balanceLifecycle
	service.catalogSync = &catalog.SyncService{}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	catalogStartErr := errors.New("injected catalog start failure")
	catalogShutdownErr := errors.New("injected catalog shutdown completion failure")
	shutdownCalls := 0
	service.catalogStart = func(context.Context) (context.Context, error) {
		return nil, catalogStartErr
	}
	service.catalogShutdown = func() error {
		shutdownCalls++
		service.catalogLifecycle.Shutdown()
		return catalogShutdownErr
	}

	runErr := service.Run(context.Background())
	if !errors.Is(runErr, catalogStartErr) {
		t.Fatalf("expected catalog start error, got %v", runErr)
	}
	if errors.Is(runErr, catalogShutdownErr) {
		t.Fatalf("catalog shutdown completion must not run after catalog start failure: %v", runErr)
	}
	if shutdownCalls != 0 {
		t.Fatalf("expected no catalog shutdown completion call after catalog start failure, got %d", shutdownCalls)
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to be stopped after rollback")
	}
	if balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to be stopped after rollback")
	}
}



type orderedCloseErrorDB struct {
	name string
	order *[]string
	err error
	closeCount int
}

func (db *orderedCloseErrorDB) Close() error {
	db.closeCount++
	if db.order != nil {
		*db.order = append(*db.order, db.name)
	}
	return db.err
}

func TestCombineRuntimeShutdownErrorPreservesDeterministicErrorOrder(t *testing.T) {
	primary := errors.New("primary shutdown error")
	worker := errors.New("worker shutdown error")
	catalogErr := errors.New("catalog shutdown error")
	transaction := errors.New("transaction close error")
	audit := errors.New("audit close error")

	err := combineRuntimeShutdownError(
		combineRuntimeShutdownError(
			combineRuntimeShutdownError(
				combineRuntimeShutdownError(primary, worker),
				catalogErr,
			),
			transaction,
		),
		audit,
	)

	want := "primary shutdown error\nworker shutdown error\ncatalog shutdown error\ntransaction close error\naudit close error"
	if err == nil || err.Error() != want {
		t.Fatalf("unexpected deterministic shutdown error order: got %q want %q", err, want)
	}
	for _, wantErr := range []error{primary, worker, catalogErr, transaction, audit} {
		if !errors.Is(err, wantErr) {
			t.Fatalf("composed shutdown error lost identity for %v: %v", wantErr, err)
		}
	}
}

func TestServiceRunShutdownPreservesCompletionOrderingAndAllErrorIdentity(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	balanceErr := errors.New("balance shutdown failed")
	catalogErr := errors.New("catalog shutdown failed")
	transactionErr := errors.New("transaction close failed")
	auditErr := errors.New("audit close failed")

	order := make([]string, 0, 4)
	var mu sync.Mutex
	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, name)
	}

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		record("balance")
		return balanceErr
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		record("catalog")
		return catalogErr
	}
	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)

	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected context cancellation identity, got %v", runErr)
	}
	if !errors.Is(runErr, balanceErr) {
		t.Fatalf("expected balance shutdown error identity, got %v", runErr)
	}
	if !errors.Is(runErr, catalogErr) {
		t.Fatalf("expected catalog shutdown error identity, got %v", runErr)
	}
	if !errors.Is(runErr, transactionErr) {
		t.Fatalf("expected transaction close error identity, got %v", runErr)
	}
	if !errors.Is(runErr, auditErr) {
		t.Fatalf("expected audit close error identity, got %v", runErr)
	}
	wantErr := "context canceled\nbalance shutdown failed\ncatalog shutdown failed\nclose transaction database: transaction close failed\nclose audit database: audit close failed"
	if runErr == nil || runErr.Error() != wantErr {
		t.Fatalf("unexpected shutdown error precedence: got %q want %q", runErr, wantErr)
	}
	if !reflect.DeepEqual(order, []string{"balance", "catalog", "transaction", "audit"}) {
		t.Fatalf("unexpected shutdown ordering: got %v", order)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected single database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	secondClose := service.Close()
	if !errors.Is(secondClose, transactionErr) || !errors.Is(secondClose, auditErr) {
		t.Fatalf("expected repeated Service.Close to preserve database cleanup errors, got %v", secondClose)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected repeated Service.Close not to double-close, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}




func TestServiceRunShutdownSerializesConcurrentCloseAndReentry(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	tx := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, nil)
	service.databaseOwnership.transferToService()

	catalogEntered := make(chan struct{})
	releaseCatalog := make(chan struct{})
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		close(catalogEntered)
		<-releaseCatalog
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan error, 1)
	go func() { firstDone <- service.Run(ctx) }()

	deadline := time.After(time.Second)
	for !service.catalogLifecycle.Running() {
		select {
		case <-deadline:
			t.Fatal("catalog lifecycle did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	cancel()
	select {
	case <-catalogEntered:
	case <-time.After(time.Second):
		t.Fatal("catalog shutdown did not enter")
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- service.Close() }()
	reentryDone := make(chan error, 1)
	go func() { reentryDone <- service.Run(context.Background()) }()

	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before shutdown completion: %v", err)
	case err := <-reentryDone:
		t.Fatalf("Run re-entry returned before shutdown completion: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if tx.closeCount != 0 {
		t.Fatalf("concurrent Close/re-entry must not close runtime database before shutdown completion, got %d", tx.closeCount)
	}

	close(releaseCatalog)

	select {
	case err := <-firstDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected first Run cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first Run did not finish")
	}

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("expected serialized Close to complete after shutdown, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serialized Close did not finish")
	}

	select {
	case err := <-reentryDone:
		if !errors.Is(err, ErrServiceClosed) {
			t.Fatalf("expected serialized Run re-entry to observe closed runtime, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serialized Run re-entry did not finish")
	}

	if tx.closeCount != 1 {
		t.Fatalf("expected runtime database close exactly once, got %d", tx.closeCount)
	}
}

func TestServiceRunShutdownErrorPrecedenceDoesNotReplayLifecycleCompletionOnRepeatedClose(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	balanceErr := errors.New("balance completion failed")
	catalogErr := errors.New("catalog completion failed")
	transactionErr := errors.New("transaction close failed")
	auditErr := errors.New("audit close failed")
	catalogCalls := 0

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		return balanceErr
	}
	service.catalogShutdown = func() error {
		catalogCalls++
		service.catalogLifecycle.Shutdown()
		return catalogErr
	}

	tx := &orderedCloseErrorDB{name: "transaction", err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)

	for _, want := range []error{context.Canceled, balanceErr, catalogErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("first shutdown error must preserve %v: %v", want, runErr)
		}
	}
	if catalogCalls != 1 {
		t.Fatalf("expected one catalog completion call during first shutdown, got %d", catalogCalls)
	}

	closeErr := service.Close()
	if errors.Is(closeErr, catalogErr) {
		t.Fatalf("repeated Close must not replay catalog completion error: %v", closeErr)
	}
	if !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("repeated Close must preserve database cleanup errors: %v", closeErr)
	}
	if catalogCalls != 1 {
		t.Fatalf("expected repeated Close not to replay catalog completion, got %d calls", catalogCalls)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected database cleanup to remain single-shot: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}

func TestServiceRunRejectsRepeatedRunAfterOwnedShutdown(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	closeErr := errors.New("owned database close failed")
	service.databaseOwnership = newRuntimeDatabaseOwnership(&closeErrorDB{err: closeErr}, nil)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	firstErr := service.Run(ctx)
	if !errors.Is(firstErr, context.Canceled) || !errors.Is(firstErr, closeErr) {
		t.Fatalf("expected first run to preserve cancellation and close errors, got %v", firstErr)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("expected first shutdown to stop balance lifecycle")
	}

	secondErr := service.Run(context.Background())
	if !errors.Is(secondErr, ErrServiceClosed) {
		t.Fatalf("expected repeated Run to return ErrServiceClosed, got %v", secondErr)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("expected repeated Run not to restart balance lifecycle")
	}

	if err := service.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("expected repeated Close to preserve original cleanup error, got %v", err)
	}
	if service.databaseOwnership.isClosed() != true {
		t.Fatal("expected runtime ownership to remain closed")
	}
}


type blockingBalanceMock struct {
	*mock.Provider
	balance int64
	entered chan struct{}
	release chan struct{}
	once sync.Once
}

func (p *blockingBalanceMock) GetBalance(context.Context) (int64, error) {
	p.once.Do(func() { close(p.entered) })
	<-p.release
	return p.balance, nil
}

func TestServiceShutdownTimeoutPreservesWorkerOwnershipBeforeDatabaseClose(t *testing.T) {
	mockProvider := &blockingBalanceMock{
		Provider: mock.New(mock.Config{}),
		balance: 100000,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	db := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
	service.databaseOwnership.transferToService()

	if err := service.balanceLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-mockProvider.entered:
	case <-time.After(time.Second):
		t.Fatal("balance worker did not enter the blocking provider call")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err = service.shutdownBalanceWorker(shutdownCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected worker shutdown deadline error, got %v", err)
	}
	if !service.balanceLifecycle.Running() {
		t.Fatal("expected balance worker to remain owned after shutdown timeout")
	}
	if closeErr := service.Close(); closeErr == nil || closeErr.Error() != "service close requires worker shutdown" {
		t.Fatalf("expected database close guard while worker remains running, got %v", closeErr)
	}
	if db.closeCount != 0 {
		t.Fatalf("expected database to remain open while worker is still running, got %d closes", db.closeCount)
	}

	close(mockProvider.release)
	if err := service.balanceLifecycle.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected worker to stop after release, got %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("expected database close after worker completion, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected one database close after worker completion, got %d", db.closeCount)
	}
}


func TestServiceRunShutdownTimeoutKeepsDatabaseOwnershipUntilWorkerStops(t *testing.T) {
	mockProvider := &blockingBalanceMock{
		Provider: mock.New(mock.Config{}),
		balance: 100000,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	registry := provider.NewRegistry()
	if err := registry.Register("mock", mockProvider); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync = nil

	db := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	runDone := make(chan error, 1)
	go func() {
		runDone <- service.Run(ctx)
	}()

	select {
	case <-mockProvider.entered:
	case <-time.After(time.Second):
		t.Fatal("balance worker did not enter the blocking provider call")
	}

	select {
	case runErr := <-runDone:
		if !errors.Is(runErr, context.DeadlineExceeded) {
			t.Fatalf("expected shutdown deadline error, got %v", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("service Run did not return after shutdown timeout")
	}

	if !service.balanceLifecycle.Running() {
		t.Fatal("expected balance worker to remain running after shutdown timeout")
	}
	if db.closeCount != 0 {
		t.Fatalf("expected database to remain open while worker is still running, got %d closes", db.closeCount)
	}
	if service.databaseOwnership.isClosed() {
		t.Fatal("expected runtime database ownership to remain open while worker is still running")
	}

	close(mockProvider.release)
	if err := service.balanceLifecycle.Shutdown(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected worker to stop while preserving its deadline error, got %v", err)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("expected worker to stop after provider release")
	}
	if err := service.Close(); err != nil {
		t.Fatalf("expected database close after worker completion, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected one database close after worker completion, got %d", db.closeCount)
	}
}

func TestServiceRunBalanceOnlyShutdownPreservesCompletionPrecedenceAndDatabaseCleanup(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync = nil

	balanceErr := errors.New("balance-only shutdown failed")
	transactionErr := errors.New("transaction close failed")
	auditErr := errors.New("audit close failed")
	order := make([]string, 0, 3)
	var mu sync.Mutex
	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, name)
	}

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		record("balance")
		return balanceErr
	}
	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)

	for _, want := range []error{context.Canceled, balanceErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected shutdown error identity for %v, got %v", want, runErr)
		}
	}
	if reflect.DeepEqual(order, []string{"balance", "transaction", "audit"}) == false {
		t.Fatalf("unexpected balance-only shutdown ordering: got %v", order)
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("catalog lifecycle must remain stopped on balance-only shutdown path")
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected single database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	if closeErr := service.Close(); !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("expected repeated Close to preserve database cleanup errors only, got %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}


func TestServiceBalanceStartFailureDoesNotCompleteUnstartedLifecycles(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync = &catalog.SyncService{}
	service.catalogLifecycle = newCatalogWorkerLifecycle()

	balanceStartErr := errors.New("injected balance start failure")
	catalogShutdownErr := errors.New("catalog must not be completed")
	balanceShutdownCalls := 0
	catalogShutdownCalls := 0
	service.balanceStart = func(context.Context) error {
		return balanceStartErr
	}
	service.balanceShutdown = func(context.Context) error {
		balanceShutdownCalls++
		return nil
	}
	service.catalogShutdown = func() error {
		catalogShutdownCalls++
		return catalogShutdownErr
	}

	runErr := service.Run(context.Background())
	if !errors.Is(runErr, balanceStartErr) {
		t.Fatalf("expected balance start error, got %v", runErr)
	}
	if errors.Is(runErr, catalogShutdownErr) {
		t.Fatalf("catalog shutdown must not run when balance never started: %v", runErr)
	}
	if balanceShutdownCalls != 0 {
		t.Fatalf("expected no balance shutdown after balance start failure, got %d", balanceShutdownCalls)
	}
	if catalogShutdownCalls != 0 {
		t.Fatalf("expected no catalog shutdown after balance start failure, got %d", catalogShutdownCalls)
	}
}


func TestServiceRunShutdownSerializesConcurrentCloseDuringBalanceCompletion(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync = nil
	service.catalogLifecycle = newCatalogWorkerLifecycle()

	tx := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, nil)
	service.databaseOwnership.transferToService()

	balanceEntered := make(chan struct{})
	releaseBalance := make(chan struct{})
	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		close(balanceEntered)
		<-releaseBalance
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- service.Run(ctx) }()

	for !service.balanceLifecycle.Running() {
		select {
		case <-time.After(time.Second):
			t.Fatal("balance lifecycle did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	cancel()
	select {
	case <-balanceEntered:
	case <-time.After(time.Second):
		t.Fatal("balance shutdown did not enter")
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- service.Close() }()

	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before balance shutdown completion: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if tx.closeCount != 0 {
		t.Fatalf("database closed before balance shutdown completion: %d", tx.closeCount)
	}

	close(releaseBalance)

	select {
	case err := <-runDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected Run cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not finish")
	}

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("expected serialized Close to complete cleanly, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not complete after shutdown")
	}

	if tx.closeCount != 1 {
		t.Fatalf("expected database close exactly once, got %d", tx.closeCount)
	}
}


func TestServiceRunShutdownCompletionOrderingAcrossWorkersAndDatabaseOwnership(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	var mu sync.Mutex
	order := make([]string, 0, 4)
	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, name)
	}

	service.balanceShutdown = func(context.Context) error {
		err := service.balanceLifecycle.Shutdown(context.Background())
		record("balance")
		return err
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		record("catalog")
		return nil
	}
	tx := &orderedCloseErrorDB{name: "transaction", order: &order}
	audit := &orderedCloseErrorDB{name: "audit", order: &order}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}

	want := []string{"balance", "catalog", "transaction", "audit"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected shutdown completion ordering: got %v want %v", order, want)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected one database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("expected repeated Close to preserve successful cleanup, got %v", err)
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("repeated Close must not replay lifecycle/database completion: got %v want %v", order, want)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("repeated Close must not double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}


func TestServiceClosedStateSeparatesRunRejectionFromRecordedCleanupError(t *testing.T) {
	cleanupErr := errors.New("recorded database cleanup failure")

	newService := func(t *testing.T) *Service {
		t.Helper()
		registry := provider.NewRegistry()
		if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
			t.Fatal(err)
		}
		syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
		if err != nil {
			t.Fatal(err)
		}
		service, err := New(syncService, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		db := &closeErrorDB{err: cleanupErr}
		service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
		service.databaseOwnership.transferToService()
		return service
	}

	t.Run("direct close", func(t *testing.T) {
		service := newService(t)
		if err := service.Close(); !errors.Is(err, cleanupErr) {
			t.Fatalf("expected direct Close to record cleanup error, got %v", err)
		}
		if err := service.Close(); !errors.Is(err, cleanupErr) {
			t.Fatalf("expected repeated Close to preserve cleanup error, got %v", err)
		}
		if err := service.Run(context.Background()); !errors.Is(err, ErrServiceClosed) {
			t.Fatalf("expected Run after direct Close to return ErrServiceClosed, got %v", err)
		} else if errors.Is(err, cleanupErr) {
			t.Fatalf("Run closed-state rejection must not replay cleanup error: %v", err)
		}
	})

	t.Run("run-owned shutdown", func(t *testing.T) {
		service := newService(t)
		service.catalogSync = nil
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		runErr := service.Run(ctx)
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected Run to preserve cancellation, got %v", runErr)
		}
		if !errors.Is(runErr, cleanupErr) {
			t.Fatalf("expected Run shutdown to preserve cleanup error, got %v", runErr)
		}
		if err := service.Close(); !errors.Is(err, cleanupErr) {
			t.Fatalf("expected repeated Close to preserve recorded cleanup error, got %v", err)
		}
		if err := service.Run(context.Background()); !errors.Is(err, ErrServiceClosed) {
			t.Fatalf("expected repeated Run after owned shutdown to return ErrServiceClosed, got %v", err)
		} else if errors.Is(err, cleanupErr) {
			t.Fatalf("Run closed-state rejection must not replay cleanup error: %v", err)
		}
	})
}


func TestServiceRunMixedShutdownErrorsPreserveIdentityAndClosedState(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	primaryErr := context.Canceled
	balanceErr := errors.New("mixed balance shutdown failure")
	catalogErr := errors.New("mixed catalog shutdown failure")
	transactionErr := errors.New("mixed transaction close failure")
	auditErr := errors.New("mixed audit close failure")

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		return balanceErr
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		return catalogErr
	}
	tx := &orderedCloseErrorDB{name: "transaction", err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)

	for _, want := range []error{primaryErr, balanceErr, catalogErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected mixed shutdown error to preserve %v, got %v", want, runErr)
		}
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected single database cleanup, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	closeErr := service.Close()
	for _, want := range []error{transactionErr, auditErr} {
		if !errors.Is(closeErr, want) {
			t.Fatalf("expected repeated Close to preserve recorded cleanup error %v, got %v", want, closeErr)
		}
	}
	for _, want := range []error{primaryErr, balanceErr, catalogErr} {
		if errors.Is(closeErr, want) {
			t.Fatalf("repeated Close must not replay lifecycle error %v: %v", want, closeErr)
		}
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("repeated Close must not double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	reentryErr := service.Run(context.Background())
	if !errors.Is(reentryErr, ErrServiceClosed) {
		t.Fatalf("expected closed-state rejection, got %v", reentryErr)
	}
	for _, want := range []error{primaryErr, balanceErr, catalogErr, transactionErr, auditErr} {
		if errors.Is(reentryErr, want) {
			t.Fatalf("closed-state rejection must not replay historical error %v: %v", want, reentryErr)
		}
	}
}



func TestServiceRunCatalogCompletionErrorDoesNotSuppressDatabaseCleanup(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	catalogErr := errors.New("catalog completion failed")
	transactionErr := errors.New("transaction cleanup failed")
	auditErr := errors.New("audit cleanup failed")
	order := make([]string, 0, 4)

	service.balanceShutdown = func(context.Context) error {
		if err := service.balanceLifecycle.Shutdown(context.Background()); err != nil {
			return err
		}
		order = append(order, "balance")
		return nil
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		order = append(order, "catalog")
		return catalogErr
	}
	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)

	for _, want := range []error{context.Canceled, catalogErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected shutdown error identity for %v, got %v", want, runErr)
		}
	}
	if want := []string{"balance", "catalog", "transaction", "audit"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected shutdown ordering: got %v want %v", order, want)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected one database cleanup each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	closeErr := service.Close()
	if !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("expected repeated Close to preserve database cleanup errors, got %v", closeErr)
	}
	if errors.Is(closeErr, catalogErr) || errors.Is(closeErr, context.Canceled) {
		t.Fatalf("repeated Close must not replay historical lifecycle/primary errors: %v", closeErr)
	}
	if !reflect.DeepEqual(order, []string{"balance", "catalog", "transaction", "audit"}) {
		t.Fatalf("repeated Close must not replay shutdown completion: got %v", order)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("repeated Close must not double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}

func TestServiceRunShutdownErrorPrecedenceRemainsStableAcrossRepeatedClose(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	primaryErr := context.Canceled
	balanceErr := errors.New("precedence balance shutdown failure")
	catalogErr := errors.New("precedence catalog shutdown failure")
	transactionErr := errors.New("precedence transaction close failure")
	auditErr := errors.New("precedence audit close failure")
	order := make([]string, 0, 4)
	record := func(name string) { order = append(order, name) }

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		record("balance")
		return balanceErr
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		record("catalog")
		return catalogErr
	}
	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)

	for _, want := range []error{primaryErr, balanceErr, catalogErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected first shutdown to preserve %v, got %v", want, runErr)
		}
	}
	wantOrder := []string{"balance", "catalog", "transaction", "audit"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("unexpected first shutdown ordering: got %v want %v", order, wantOrder)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected one database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	closeErr := service.Close()
	for _, want := range []error{transactionErr, auditErr} {
		if !errors.Is(closeErr, want) {
			t.Fatalf("expected repeated Close to preserve %v, got %v", want, closeErr)
		}
	}
	for _, want := range []error{primaryErr, balanceErr, catalogErr} {
		if errors.Is(closeErr, want) {
			t.Fatalf("repeated Close must not replay lifecycle error %v: %v", want, closeErr)
		}
	}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("repeated Close must not replay shutdown completion: got %v want %v", order, wantOrder)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("repeated Close must not double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	if err := service.Run(context.Background()); !errors.Is(err, ErrServiceClosed) {
		t.Fatalf("expected closed-state rejection after completed shutdown, got %v", err)
	}
}


func TestServiceRunDoesNotCloseDatabasesWhileCatalogCompletionLeavesLifecycleRunning(t *testing.T) {
	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service.catalogSync, err = catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	catalogErr := errors.New("catalog completion failed while lifecycle remains active")
	tx := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, nil)
	service.databaseOwnership.transferToService()

	service.catalogShutdown = func() error {
		return catalogErr
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)

	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected cancellation error, got %v", runErr)
	}
	if !errors.Is(runErr, catalogErr) {
		t.Fatalf("expected catalog completion error, got %v", runErr)
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to remain running in this injected failure scenario")
	}
	if tx.closeCount != 0 {
		t.Fatalf("database must not close while catalog lifecycle remains active, got %d closes", tx.closeCount)
	}

	closeErr := service.Close()
	if closeErr == nil {
		t.Fatal("expected explicit Close to reject an active catalog lifecycle")
	}
	if tx.closeCount != 0 {
		t.Fatalf("explicit Close must not close database while catalog lifecycle remains active, got %d closes", tx.closeCount)
	}
}
