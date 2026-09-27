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



type runtimeTypedShutdownError struct{ stage string }

func (e *runtimeTypedShutdownError) Error() string { return "runtime shutdown " + e.stage + " error" }

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

func TestCombineRuntimeShutdownErrorPreservesTypedIdentityAcrossMixedFailures(t *testing.T) {
	primary := &runtimeTypedShutdownError{stage: "primary"}
	worker := &runtimeTypedShutdownError{stage: "worker"}
	catalogErr := &runtimeTypedShutdownError{stage: "catalog"}
	transaction := &runtimeTypedShutdownError{stage: "transaction"}
	audit := &runtimeTypedShutdownError{stage: "audit"}

	err := combineRuntimeShutdownError(
		combineRuntimeShutdownError(
			combineRuntimeShutdownError(
				combineRuntimeShutdownError(primary, worker),
				catalogErr,
			),
			fmt.Errorf("close transaction database: %w", transaction),
		),
		fmt.Errorf("close audit database: %w", audit),
	)

	for _, want := range []*runtimeTypedShutdownError{primary, worker, catalogErr, transaction, audit} {
		if !errors.Is(err, want) {
			t.Fatalf("composed shutdown error lost typed identity for %q: %v", want.stage, err)
		}
	}

	var got *runtimeTypedShutdownError
	if !errors.As(err, &got) {
		t.Fatalf("composed shutdown error lost errors.As support: %v", err)
	}
	if got != primary {
		t.Fatalf("errors.As must preserve the first typed error in the joined chain, got %q want %q", got.stage, primary.stage)
	}
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


func TestServiceRunCleanupFailureClosesOwnershipAndRejectsReentryWithoutReplay(t *testing.T) {
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

	transactionErr := errors.New("transaction ownership cleanup failed")
	auditErr := errors.New("audit ownership cleanup failed")
	tx := &orderedCloseErrorDB{name: "transaction", err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	firstErr := service.Run(ctx)

	for _, want := range []error{context.Canceled, transactionErr, auditErr} {
		if !errors.Is(firstErr, want) {
			t.Fatalf("expected first shutdown to preserve %v, got %v", want, firstErr)
		}
	}
	if !service.databaseOwnership.isClosed() {
		t.Fatal("expected runtime ownership to be closed after cleanup attempt")
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected each database to close once, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to stop after shutdown")
	}

	secondErr := service.Run(context.Background())
	if !errors.Is(secondErr, ErrServiceClosed) {
		t.Fatalf("expected re-entry to reject closed service, got %v", secondErr)
	}
	for _, historical := range []error{context.Canceled, transactionErr, auditErr} {
		if errors.Is(secondErr, historical) {
			t.Fatalf("closed-state rejection must not replay historical error %v: %v", historical, secondErr)
		}
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("re-entry must not double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	closeErr := service.Close()
	for _, want := range []error{transactionErr, auditErr} {
		if !errors.Is(closeErr, want) {
			t.Fatalf("repeated Close must preserve recorded cleanup error %v, got %v", want, closeErr)
		}
	}
	if errors.Is(closeErr, context.Canceled) {
		t.Fatalf("repeated Close must not replay primary Run error: %v", closeErr)
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


func TestServiceRunRejectsReentryWhilePartialCatalogShutdownRemainsActive(t *testing.T) {
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

	catalogErr := errors.New("catalog shutdown left lifecycle active")
	service.catalogShutdown = func() error {
		return catalogErr
	}
	db := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runErr := service.Run(ctx)
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", runErr)
	}
	if !errors.Is(runErr, catalogErr) {
		t.Fatalf("expected partial catalog shutdown error, got %v", runErr)
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to remain active")
	}
	if db.closeCount != 0 {
		t.Fatalf("expected database ownership to remain open, got %d closes", db.closeCount)
	}

	reentryErr := service.Run(context.Background())
	if !errors.Is(reentryErr, ErrServiceLifecycleActive) {
		t.Fatalf("expected partial-shutdown re-entry rejection, got %v", reentryErr)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("re-entry rejection must not restart the balance lifecycle")
	}
	if db.closeCount != 0 {
		t.Fatalf("re-entry rejection must not close database ownership, got %d closes", db.closeCount)
	}

	service.catalogLifecycle.Shutdown()
	if err := service.Close(); err != nil {
		t.Fatalf("expected cleanup after lifecycle convergence, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected database ownership to close exactly once after convergence, got %d", db.closeCount)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("expected repeated Close to remain idempotent, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close, got %d", db.closeCount)
	}
}


func TestServiceRunShutdownErrorCompositionPreservesOrdering(t *testing.T) {
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
	balanceErr := errors.New("ordered balance shutdown failure")
	catalogErr := errors.New("ordered catalog shutdown failure")
	transactionErr := errors.New("ordered transaction cleanup failure")
	auditErr := errors.New("ordered audit cleanup failure")

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
			t.Fatalf("expected composed shutdown error to preserve %v, got %v", want, runErr)
		}
	}

	message := runErr.Error()
	orderedNeedles := []string{
		"ordered balance shutdown failure",
		"ordered catalog shutdown failure",
		"close transaction database: ordered transaction cleanup failure",
		"close audit database: ordered audit cleanup failure",
	}
	last := -1
	for _, needle := range orderedNeedles {
		index := strings.Index(message, needle)
		if index < 0 {
			t.Fatalf("expected composed error to contain %q: %q", needle, message)
		}
		if index <= last {
			t.Fatalf("expected composed error ordering to remain stable, got %q", message)
		}
		last = index
	}

	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected one database cleanup each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}


func TestServiceRunShutdownErrorCompositionPreservesPrimaryThenLifecycleThenDatabaseOrder(t *testing.T) {
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
	balanceErr := errors.New("primary-order balance shutdown failure")
	catalogErr := errors.New("primary-order catalog shutdown failure")
	transactionErr := errors.New("primary-order transaction cleanup failure")
	auditErr := errors.New("primary-order audit cleanup failure")

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

	orderedNeedles := []string{
		"context canceled",
		"primary-order balance shutdown failure",
		"primary-order catalog shutdown failure",
		"close transaction database: primary-order transaction cleanup failure",
		"close audit database: primary-order audit cleanup failure",
	}
	last := -1
	for _, needle := range orderedNeedles {
		index := strings.Index(runErr.Error(), needle)
		if index < 0 {
			t.Fatalf("expected composed error to contain %q: %v", needle, runErr)
		}
		if index <= last {
			t.Fatalf("expected primary/lifecycle/database precedence order, got %q", runErr)
		}
		last = index
	}
	for _, want := range []error{primaryErr, balanceErr, catalogErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected composed shutdown error to preserve %v, got %v", want, runErr)
		}
	}

	closeErr := service.Close()
	for _, want := range []error{transactionErr, auditErr} {
		if !errors.Is(closeErr, want) {
			t.Fatalf("expected repeated Close to preserve database cleanup error %v, got %v", want, closeErr)
		}
	}
	for _, want := range []error{primaryErr, balanceErr, catalogErr} {
		if errors.Is(closeErr, want) {
			t.Fatalf("repeated Close must not replay lifecycle error %v: %v", want, closeErr)
		}
	}
}


func TestServiceRecordedCleanupErrorPersistsAcrossRepeatedCloseAndReentry(t *testing.T) {
	t.Run("direct close", func(t *testing.T) {
		cleanupErr := errors.New("persistent direct cleanup failure")
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

		firstClose := service.Close()
		secondClose := service.Close()
		if firstClose == nil || secondClose == nil {
			t.Fatal("expected both Close calls to return the recorded cleanup error")
		}
		if firstClose != secondClose {
			t.Fatalf("expected repeated Close to return the same recorded error object, got first=%p second=%p", firstClose, secondClose)
		}
		if !errors.Is(firstClose, cleanupErr) || !errors.Is(secondClose, cleanupErr) {
			t.Fatalf("expected repeated Close errors to preserve cleanup identity, first=%v second=%v", firstClose, secondClose)
		}
		if db.closeCount != 1 {
			t.Fatalf("expected direct-close ownership cleanup exactly once, got %d", db.closeCount)
		}

		reentryErr := service.Run(context.Background())
		if !errors.Is(reentryErr, ErrServiceClosed) {
			t.Fatalf("expected closed-state re-entry rejection, got %v", reentryErr)
		}
		if errors.Is(reentryErr, cleanupErr) {
			t.Fatalf("re-entry rejection must not replay recorded cleanup error: %v", reentryErr)
		}
	})

	t.Run("run-owned shutdown", func(t *testing.T) {
		cleanupErr := errors.New("persistent run-owned cleanup failure")
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
		db := &closeErrorDB{err: cleanupErr}
		service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
		service.databaseOwnership.transferToService()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		runErr := service.Run(ctx)
		if !errors.Is(runErr, context.Canceled) || !errors.Is(runErr, cleanupErr) {
			t.Fatalf("expected Run to preserve primary and cleanup errors, got %v", runErr)
		}

		firstClose := service.Close()
		secondClose := service.Close()
		if firstClose == nil || secondClose == nil {
			t.Fatal("expected repeated Close to return the recorded cleanup error")
		}
		if firstClose != secondClose {
			t.Fatalf("expected repeated Close to return the same recorded error object, got first=%p second=%p", firstClose, secondClose)
		}
		if !errors.Is(firstClose, cleanupErr) || !errors.Is(secondClose, cleanupErr) {
			t.Fatalf("expected repeated Close errors to preserve cleanup identity, first=%v second=%v", firstClose, secondClose)
		}
		if errors.Is(firstClose, context.Canceled) || errors.Is(secondClose, context.Canceled) {
			t.Fatal("repeated Close must not replay the Run primary error")
		}
		if db.closeCount != 1 {
			t.Fatalf("expected Run-owned cleanup exactly once, got %d", db.closeCount)
		}

		reentryErr := service.Run(context.Background())
		if !errors.Is(reentryErr, ErrServiceClosed) {
			t.Fatalf("expected closed-state re-entry rejection, got %v", reentryErr)
		}
		if errors.Is(reentryErr, cleanupErr) || errors.Is(reentryErr, context.Canceled) {
			t.Fatalf("re-entry rejection must not replay historical shutdown errors: %v", reentryErr)
		}
	})
}


func TestRuntimeOwnershipRetainsCleanupErrorsAcrossSharedAndDedicatedTopologies(t *testing.T) {
	tests := []struct {
		name      string
		shared    bool
		runOwned  bool
	}{
		{name: "shared-direct-close", shared: true},
		{name: "shared-run-owned", shared: true, runOwned: true},
		{name: "dedicated-direct-close", shared: false},
		{name: "dedicated-run-owned", shared: false, runOwned: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactionErr := errors.New("transaction cleanup failure")
			auditErr := errors.New("audit cleanup failure")

			tx := &closeErrorDB{err: transactionErr}
			var audit *closeErrorDB
			if tt.shared {
				audit = tx
			} else {
				audit = &closeErrorDB{err: auditErr}
			}

			service := newRuntimeTestService(t)
			service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
			service.databaseOwnership.transferToService()

			var firstErr error
			if tt.runOwned {
				service.catalogSync = nil
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				firstErr = service.Run(ctx)
				if !errors.Is(firstErr, context.Canceled) {
					t.Fatalf("expected Run-owned cancellation identity, got %v", firstErr)
				}
			} else {
				firstErr = service.Close()
			}

			if !errors.Is(firstErr, transactionErr) {
				t.Fatalf("expected transaction cleanup error identity, got %v", firstErr)
			}
			if !tt.shared && !errors.Is(firstErr, auditErr) {
				t.Fatalf("expected dedicated audit cleanup error identity, got %v", firstErr)
			}

			secondErr := service.Close()
			if secondErr == nil {
				t.Fatal("expected repeated Close to retain recorded cleanup error")
			}
			if secondErr != service.databaseOwnership.closeErr {
				t.Fatalf("expected repeated Close to return the stored ownership cleanup result object")
			}
			if !errors.Is(secondErr, transactionErr) {
				t.Fatalf("expected repeated Close to preserve transaction cleanup error, got %v", secondErr)
			}
			if !tt.shared && !errors.Is(secondErr, auditErr) {
				t.Fatalf("expected repeated Close to preserve dedicated audit cleanup error, got %v", secondErr)
			}

			if tx.closeCount != 1 {
				t.Fatalf("expected transaction handle to close exactly once, got %d", tx.closeCount)
			}
			if tt.shared {
				if audit.closeCount != 1 {
					t.Fatalf("expected shared handle to close exactly once, got %d", audit.closeCount)
				}
			} else if audit.closeCount != 1 {
				t.Fatalf("expected dedicated audit handle to close exactly once, got %d", audit.closeCount)
			}

			reentryErr := service.Run(context.Background())
			if !errors.Is(reentryErr, ErrServiceClosed) {
				t.Fatalf("expected closed-state re-entry rejection, got %v", reentryErr)
			}
			if errors.Is(reentryErr, transactionErr) || errors.Is(reentryErr, auditErr) {
				t.Fatalf("closed-state re-entry must not replay historical cleanup errors: %v", reentryErr)
			}
		})
	}
}

func newRuntimeTestService(t *testing.T) *Service {
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
	return service
}



func TestServiceRunDoesNotCloseDatabasesWhileBalanceCompletionLeavesLifecycleRunning(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	balanceErr := errors.New("balance completion failed while lifecycle remains active")
	catalogErr := errors.New("catalog completion failed after balance failure")
	transactionErr := errors.New("transaction close must be deferred")
	auditErr := errors.New("audit close must be deferred")
	order := make([]string, 0, 4)

	service.balanceStart = func(context.Context) error {
		return service.balanceLifecycle.Start(context.Background())
	}
	service.balanceShutdown = func(context.Context) error {
		order = append(order, "balance")
		return balanceErr
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

	for _, want := range []error{context.Canceled, balanceErr, catalogErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected shutdown error identity for %v, got %v", want, runErr)
		}
	}
	for _, unexpected := range []error{transactionErr, auditErr} {
		if errors.Is(runErr, unexpected) {
			t.Fatalf("database cleanup must not run while balance lifecycle remains active: %v", runErr)
		}
	}

	if !reflect.DeepEqual(order, []string{"balance", "catalog"}) {
		t.Fatalf("unexpected partial shutdown ordering: got %v", order)
	}
	if !service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to remain active after injected completion failure")
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to complete after balance completion failure")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected database ownership to remain open, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	closeErr := service.Close()
	if closeErr == nil {
		t.Fatal("expected Close to reject while balance lifecycle remains active")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("Close must not close database ownership while balance lifecycle remains active, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	service.balanceLifecycle.Shutdown(context.Background())
	if closeErr = service.Close(); !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("expected converged Close to return recorded database cleanup errors, got %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected converged ownership cleanup exactly once, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{"balance", "catalog", "transaction", "audit"}) {
		t.Fatalf("unexpected final cleanup ordering: got %v", order)
	}
}

func TestServiceRepeatedLifecycleCompletionPreservesOrderingAndSingleClose(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()

	balanceErr := errors.New("repeated balance shutdown error")
	catalogErr := errors.New("repeated catalog shutdown error")
	balanceCalls := 0
	catalogCalls := 0

	service.balanceShutdown = func(context.Context) error {
		balanceCalls++
		service.balanceLifecycle.Shutdown(context.Background())
		return balanceErr
	}
	service.catalogShutdown = func() error {
		catalogCalls++
		service.catalogLifecycle.Shutdown()
		return catalogErr
	}

	if err := service.balanceLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.catalogLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := service.shutdownBalanceWorker(context.Background()); !errors.Is(err, balanceErr) {
		t.Fatalf("expected first balance completion error, got %v", err)
	}
	if err := service.shutdownBalanceWorker(context.Background()); err != nil {
		t.Fatalf("expected completed balance lifecycle to be a no-op, got %v", err)
	}
	if err := service.shutdownCatalogLifecycle(); !errors.Is(err, catalogErr) {
		t.Fatalf("expected first catalog completion error, got %v", err)
	}
	if err := service.shutdownCatalogLifecycle(); err != nil {
		t.Fatalf("expected completed catalog lifecycle to be a no-op, got %v", err)
	}

	if balanceCalls != 1 || catalogCalls != 1 {
		t.Fatalf("expected completed lifecycle shutdown seams to remain single-shot, got balance=%d catalog=%d", balanceCalls, catalogCalls)
	}
	if service.balanceLifecycle.Running() || service.catalogLifecycle.Running() {
		t.Fatal("expected repeated completion to leave both lifecycles stopped")
	}
}

func TestServiceRunShutdownDefersDatabaseCloseUntilAllLifecyclesStop(t *testing.T) {
	workerErr := errors.New("injected shutdown worker error")
	catalogErr := errors.New("injected shutdown catalog error")
	transactionCleanupErr := errors.New("injected deferred transaction cleanup error")
	auditCleanupErr := errors.New("injected deferred audit cleanup error")
	cleanupOrder := []string{}

	registry := provider.NewRegistry()
	balanceProvider := &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}
	if err := registry.Register("mock", balanceProvider); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	catalogStore, err := catalog.NewJSONFileStore(filepath.Join(t.TempDir(), "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalogSync, err := catalog.NewSyncService(registry, catalogStore)
	if err != nil {
		t.Fatal(err)
	}
	balanceLifecycle, err := operational.NewSyncWorkerLifecycle(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	transactionDB := &runtimeCleanupErrorDB{err: transactionCleanupErr, name: "transaction", order: &cleanupOrder}
	auditDB := &runtimeCleanupErrorDB{err: auditCleanupErr, name: "audit", order: &cleanupOrder}
	ownership := newRuntimeDatabaseOwnership(transactionDB, auditDB)
	ownership.transferToService()

	service := &Service{
		syncService:       syncService,
		catalogSync:       catalogSync,
		databaseOwnership: ownership,
		balanceLifecycle:  balanceLifecycle,
		catalogLifecycle:  newCatalogWorkerLifecycle(),
		interval:          time.Hour,
		catalogInterval:   time.Hour,
		balanceShutdown: func(context.Context) error {
			if err := balanceLifecycle.Shutdown(context.Background()); err != nil {
				return errors.Join(workerErr, err)
			}
			return workerErr
		},
		catalogShutdown: func() error {
			return catalogErr
		},
	}
	service.balanceStart = func(context.Context) error {
		return service.balanceLifecycle.Start(context.Background())
	}
	service.catalogStart = func(context.Context) (context.Context, error) {
		return service.catalogLifecycle.Start(context.Background())
	}

	runCtx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- service.Run(runCtx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for lifecycles to start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected context cancellation as primary error, got %v", runErr)
		}
		if !errors.Is(runErr, workerErr) {
			t.Fatalf("expected worker shutdown error, got %v", runErr)
		}
		if !errors.Is(runErr, catalogErr) {
			t.Fatalf("expected catalog shutdown error, got %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown completion")
	}

	if transactionDB.closeCount != 0 || auditDB.closeCount != 0 {
		t.Fatalf("database cleanup must be deferred while catalog lifecycle remains active: tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount)
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to remain active after failed completion")
	}
	if balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to stop before deferred database cleanup")
	}

	service.catalogLifecycle.Shutdown()
	closeErr := service.Close()
	if !errors.Is(closeErr, transactionCleanupErr) || !errors.Is(closeErr, auditCleanupErr) {
		t.Fatalf("expected deferred database cleanup errors after lifecycle completion, got %v", closeErr)
	}
	if transactionDB.closeCount != 1 || auditDB.closeCount != 1 {
		t.Fatalf("expected each database to close exactly once after all lifecycles stop: tx=%d audit=%d", transactionDB.closeCount, auditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected transaction-before-audit cleanup order, got %v", cleanupOrder)
	}
}


func TestServiceRunReentryAfterPartialShutdownConvergesToTerminalState(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	partialCatalogErr := errors.New("partial catalog shutdown error")
	cleanupErr := errors.New("terminal database cleanup error")
	db := &closeErrorDB{err: cleanupErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
	service.databaseOwnership.transferToService()

	service.catalogShutdown = func() error {
		return partialCatalogErr
	}

	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstCancel()
	firstErr := service.Run(firstCtx)
	if !errors.Is(firstErr, context.Canceled) {
		t.Fatalf("expected first shutdown cancellation, got %v", firstErr)
	}
	if !errors.Is(firstErr, partialCatalogErr) {
		t.Fatalf("expected first partial catalog shutdown error, got %v", firstErr)
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to remain active after partial shutdown")
	}
	if db.closeCount != 0 {
		t.Fatalf("expected database ownership to remain open during partial shutdown, got %d closes", db.closeCount)
	}

	reentryErr := service.Run(context.Background())
	if !errors.Is(reentryErr, ErrServiceLifecycleActive) {
		t.Fatalf("expected active-lifecycle re-entry rejection, got %v", reentryErr)
	}
	if errors.Is(reentryErr, partialCatalogErr) {
		t.Fatalf("re-entry rejection must not replay historical catalog error: %v", reentryErr)
	}

	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		return nil
	}
	if err := service.shutdownCatalogLifecycle(); err != nil {
		t.Fatalf("expected lifecycle convergence without error, got %v", err)
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to converge to stopped state")
	}

	secondCtx, secondCancel := context.WithCancel(context.Background())
	secondCancel()
	secondErr := service.Run(secondCtx)
	if !errors.Is(secondErr, context.Canceled) {
		t.Fatalf("expected second shutdown cancellation, got %v", secondErr)
	}
	if errors.Is(secondErr, partialCatalogErr) {
		t.Fatalf("second Run must not replay historical catalog error: %v", secondErr)
	}
	if !errors.Is(secondErr, cleanupErr) {
		t.Fatalf("second Run must preserve the new terminal cleanup error, got %v", secondErr)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected terminal Run shutdown to close database ownership once, got %d", db.closeCount)
	}

	closeErr := service.Close()
	if !errors.Is(closeErr, cleanupErr) {
		t.Fatalf("expected repeated Close to preserve terminal cleanup error, got %v", closeErr)
	}
	if errors.Is(closeErr, partialCatalogErr) || errors.Is(closeErr, context.Canceled) {
		t.Fatalf("terminal Close must not replay historical lifecycle/primary errors: %v", closeErr)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected terminal Close not to double-close database ownership, got %d", db.closeCount)
	}

	terminalErr := service.Run(context.Background())
	if !errors.Is(terminalErr, ErrServiceClosed) {
		t.Fatalf("expected terminal closed-state rejection, got %v", terminalErr)
	}
	if errors.Is(terminalErr, partialCatalogErr) || errors.Is(terminalErr, cleanupErr) {
		t.Fatalf("terminal re-entry must not replay historical shutdown errors: %v", terminalErr)
	}
}


func TestServiceRunShutdownDeadlineDefersDatabaseCleanupWhileBalanceLifecycleRemainsActive(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	shutdownDeadlineErr := context.DeadlineExceeded
	transactionErr := errors.New("deadline-deferred transaction cleanup")
	auditErr := errors.New("deadline-deferred audit cleanup")
	order := make([]string, 0, 4)

	service.balanceStart = func(context.Context) error {
		return service.balanceLifecycle.Start(context.Background())
	}
	service.balanceShutdown = func(ctx context.Context) error {
		if ctx == nil {
			t.Fatal("shutdown context must not be nil")
		}
		<-ctx.Done()
		if err := ctx.Err(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected shutdown deadline, got %v", err)
		}
		return shutdownDeadlineErr
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		order = append(order, "catalog")
		return nil
	}

	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	runErr := service.Run(ctx)
	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("expected primary deadline error, got %v", runErr)
	}
	if !errors.Is(runErr, shutdownDeadlineErr) {
		t.Fatalf("expected shutdown deadline error, got %v", runErr)
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("database cleanup must remain deferred while balance lifecycle is active: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to remain active after deadline-bound shutdown failure")
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to stop despite balance shutdown deadline")
	}
	if !reflect.DeepEqual(order, []string{"catalog"}) {
		t.Fatalf("expected only catalog completion before deferred database cleanup, got %v", order)
	}

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		order = append(order, "balance")
		return nil
	}
	if err := service.shutdownBalanceWorker(context.Background()); err != nil {
		t.Fatalf("expected balance lifecycle convergence, got %v", err)
	}
	closeErr := service.Close()
	if !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("expected deferred database cleanup errors after lifecycle convergence, got %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected each database to close exactly once, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{"catalog", "balance", "transaction", "audit"}) {
		t.Fatalf("unexpected final completion ordering: %v", order)
	}
}


func TestServiceRunShutdownCancellationVsLifecycleCompletionPrecedence(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	balanceErr := errors.New("balance completion deadline")
	catalogErr := errors.New("catalog completion cancellation")
	transactionErr := errors.New("transaction cleanup after cancellation")
	auditErr := errors.New("audit cleanup after cancellation")
	order := make([]string, 0, 4)

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		order = append(order, "balance")
		return balanceErr
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
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected caller cancellation identity, got %v", runErr)
	}
	for _, want := range []error{balanceErr, catalogErr, transactionErr, auditErr} {
		if !errors.Is(runErr, want) {
			t.Fatalf("expected composed shutdown error identity %v, got %v", want, runErr)
		}
	}
	wantErr := "context canceled\nbalance completion deadline\ncatalog completion cancellation\nclose transaction database: transaction cleanup after cancellation\nclose audit database: audit cleanup after cancellation"
	if runErr == nil || runErr.Error() != wantErr {
		t.Fatalf("unexpected shutdown error precedence: got %q want %q", runErr, wantErr)
	}
	if !reflect.DeepEqual(order, []string{"balance", "catalog", "transaction", "audit"}) {
		t.Fatalf("unexpected shutdown completion ordering: got %v", order)
	}
	if service.balanceLifecycle.Running() || service.catalogLifecycle.Running() {
		t.Fatal("expected both lifecycles to converge before database cleanup")
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected one database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	repeatedCloseErr := service.Close()
	for _, want := range []error{transactionErr, auditErr} {
		if !errors.Is(repeatedCloseErr, want) {
			t.Fatalf("expected repeated Close to preserve cleanup error %v, got %v", want, repeatedCloseErr)
		}
	}
	if errors.Is(repeatedCloseErr, context.Canceled) ||
		errors.Is(repeatedCloseErr, balanceErr) ||
		errors.Is(repeatedCloseErr, catalogErr) {
		t.Fatalf("repeated Close must not replay historical shutdown errors: %v", repeatedCloseErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}



func TestServiceRunDeferredOwnershipCleanupPreservesFreshCloseErrors(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	historicalPrimaryErr := errors.New("historical cancellation")
	historicalCatalogErr := errors.New("historical catalog completion")
	transactionErr := errors.New("fresh transaction cleanup")
	auditErr := errors.New("fresh audit cleanup")
	order := make([]string, 0, 4)

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		order = append(order, "balance")
		return nil
	}
	service.catalogShutdown = func() error {
		order = append(order, "catalog")
		return historicalCatalogErr
	}

	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Preserve a distinct primary error through a canceled Run while the
	// catalog lifecycle remains active, forcing database ownership cleanup
	// to remain deferred.
	service.balanceStart = func(context.Context) error {
		return service.balanceLifecycle.Start(context.Background())
	}
	service.balanceStart = func(context.Context) error {
		return service.balanceLifecycle.Start(context.Background())
	}
	service.catalogStart = func(context.Context) (context.Context, error) {
		return service.catalogLifecycle.Start(context.Background())
	}

	runErr := service.Run(ctx)
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected historical primary cancellation, got %v", runErr)
	}
	if !errors.Is(runErr, historicalCatalogErr) {
		t.Fatalf("expected historical catalog completion error, got %v", runErr)
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected database cleanup to remain deferred, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to remain active after partial shutdown")
	}

	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		order = append(order, "catalog-converged")
		return nil
	}
	if err := service.shutdownCatalogLifecycle(); err != nil {
		t.Fatalf("expected catalog convergence without error, got %v", err)
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to converge to stopped state")
	}

	closeErr := service.Close()
	if !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("expected fresh database cleanup errors, got %v", closeErr)
	}
	if errors.Is(closeErr, historicalPrimaryErr) ||
		errors.Is(closeErr, historicalCatalogErr) ||
		errors.Is(closeErr, context.Canceled) {
		t.Fatalf("explicit Close must not replay historical Run errors: %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected exactly one close per database, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{"balance", "catalog", "catalog-converged", "transaction", "audit"}) {
		t.Fatalf("unexpected deferred cleanup ordering: %v", order)
	}
}


func TestServiceRunCloseReentryInterleavingsRemainSingleShot(t *testing.T) {
	service := newRuntimeTestService(t)
	db := &closeErrorDB{}
	service.databaseOwnership = newRuntimeDatabaseOwnership(db, nil)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- service.Run(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !service.balanceLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for balance lifecycle to start")
		}
		time.Sleep(time.Millisecond)
	}

	if err := service.Close(); err == nil {
		t.Fatal("expected Close to reject while balance lifecycle is active")
	}
	if db.closeCount != 0 {
		t.Fatalf("active lifecycle Close must not close runtime database, got %d closes", db.closeCount)
	}

	if err := service.Run(context.Background()); !errors.Is(err, operational.ErrSyncWorkerRunning) {
		t.Fatalf("expected concurrent Run to reject active balance lifecycle, got %v", err)
	}
	if db.closeCount != 0 {
		t.Fatalf("rejected concurrent Run must not close runtime database, got %d closes", db.closeCount)
	}

	cancel()
	select {
	case runErr := <-runDone:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected owned Run shutdown to preserve cancellation, got %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run shutdown")
	}

	if service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to stop before terminal Close")
	}
	if db.closeCount != 1 {
		t.Fatalf("expected terminal Run shutdown to close runtime database once, got %d closes", db.closeCount)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("expected repeated Close to remain idempotent, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close runtime database, got %d closes", db.closeCount)
	}

	if err := service.Run(context.Background()); !errors.Is(err, ErrServiceClosed) {
		t.Fatalf("expected terminal Run re-entry to return ErrServiceClosed, got %v", err)
	}
	if db.closeCount != 1 {
		t.Fatalf("terminal Run re-entry must not reuse or close runtime database, got %d closes", db.closeCount)
	}
}



func TestServiceFreshRunAfterPartialLifecycleConvergenceDoesNotReplayHistoricalErrors(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	historicalCatalogErr := errors.New("historical catalog completion failure")
	freshTransactionErr := errors.New("fresh transaction cleanup failure")
	freshAuditErr := errors.New("fresh audit cleanup failure")
	order := make([]string, 0, 8)
	catalogCalls := 0

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		order = append(order, "balance")
		return nil
	}
	service.catalogShutdown = func() error {
		catalogCalls++
		order = append(order, "catalog")
		if catalogCalls == 1 {
			return historicalCatalogErr
		}
		service.catalogLifecycle.Shutdown()
		return nil
	}
	service.catalogStart = func(parent context.Context) (context.Context, error) {
		return service.catalogLifecycle.Start(parent)
	}

	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: freshTransactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: freshAuditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() { firstDone <- service.Run(firstCtx) }()
	deadline := time.Now().Add(5 * time.Second)
	for !service.balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		if time.Now().After(deadline) {
			firstCancel()
			t.Fatal("timed out waiting for both lifecycles to start")
		}
		time.Sleep(time.Millisecond)
	}
	firstCancel()
	firstErr := <-firstDone
	if !errors.Is(firstErr, context.Canceled) || !errors.Is(firstErr, historicalCatalogErr) {
		t.Fatalf("expected first Run to preserve cancellation and historical catalog error, got %v", firstErr)
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to remain active after partial first shutdown")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected ownership cleanup to remain deferred after partial first shutdown: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	if err := service.shutdownCatalogLifecycle(); err != nil {
		t.Fatalf("expected catalog convergence before fresh Run, got %v", err)
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to converge before fresh Run")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected database ownership to remain open before fresh Run: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	secondCtx, secondCancel := context.WithCancel(context.Background())
	secondCancel()
	secondErr := service.Run(secondCtx)
	if !errors.Is(secondErr, context.Canceled) {
		t.Fatalf("expected fresh Run to preserve fresh cancellation, got %v", secondErr)
	}
	if errors.Is(secondErr, historicalCatalogErr) {
		t.Fatalf("fresh Run must not replay historical catalog error: %v", secondErr)
	}
	if !errors.Is(secondErr, freshTransactionErr) || !errors.Is(secondErr, freshAuditErr) {
		t.Fatalf("expected fresh database cleanup errors, got %v", secondErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected fresh Run to close each database exactly once, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{
		"balance",
		"catalog",
		"catalog",
		"balance",
		"catalog",
		"transaction",
		"audit",
	}) {
		t.Fatalf("unexpected fresh-run completion ordering: %v", order)
	}

	closeErr := service.Close()
	if !errors.Is(closeErr, freshTransactionErr) || !errors.Is(closeErr, freshAuditErr) {
		t.Fatalf("expected repeated Close to preserve fresh cleanup errors, got %v", closeErr)
	}
	if errors.Is(closeErr, historicalCatalogErr) || errors.Is(closeErr, context.Canceled) {
		t.Fatalf("repeated Close must not replay historical or primary Run errors: %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}

func TestServiceRunBothLifecyclesRemainActiveThenConvergeBeforeFreshCloseCleanup(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	balanceShutdownErr := errors.New("first balance shutdown did not converge")
	catalogShutdownErr := errors.New("first catalog shutdown did not converge")
	transactionErr := errors.New("fresh transaction cleanup after convergence")
	auditErr := errors.New("fresh audit cleanup after convergence")
	order := make([]string, 0, 4)

	service.balanceShutdown = func(context.Context) error {
		order = append(order, "balance-attempt")
		return balanceShutdownErr
	}
	service.catalogShutdown = func() error {
		order = append(order, "catalog-attempt")
		return catalogShutdownErr
	}
	service.balanceStart = func(context.Context) error {
		return service.balanceLifecycle.Start(context.Background())
	}
	service.catalogStart = func(context.Context) (context.Context, error) {
		return service.catalogLifecycle.Start(context.Background())
	}

	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- service.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for !service.balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for both lifecycles to start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	runErr := <-runDone

	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected first Run to preserve caller cancellation, got %v", runErr)
	}
	if !errors.Is(runErr, balanceShutdownErr) {
		t.Fatalf("expected first Run to preserve balance completion error, got %v", runErr)
	}
	if !errors.Is(runErr, catalogShutdownErr) {
		t.Fatalf("expected first Run to preserve catalog completion error, got %v", runErr)
	}
	if service.balanceLifecycle == nil || !service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to remain active after first failed convergence")
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to remain active after first failed convergence")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected database cleanup to remain deferred while both lifecycles are active: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{"balance-attempt", "catalog-attempt"}) {
		t.Fatalf("unexpected first shutdown attempt order: %v", order)
	}

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		order = append(order, "balance-converged")
		return nil
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		order = append(order, "catalog-converged")
		return nil
	}

	if err := service.shutdownBalanceWorker(context.Background()); err != nil {
		t.Fatalf("expected balance convergence without error, got %v", err)
	}
	if err := service.shutdownCatalogLifecycle(); err != nil {
		t.Fatalf("expected catalog convergence without error, got %v", err)
	}
	if service.balanceLifecycle.Running() || service.catalogLifecycle.Running() {
		t.Fatal("expected both lifecycles to be stopped before explicit Close")
	}

	closeErr := service.Close()
	if !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("expected fresh database cleanup errors after lifecycle convergence, got %v", closeErr)
	}
	if errors.Is(closeErr, context.Canceled) ||
		errors.Is(closeErr, balanceShutdownErr) ||
		errors.Is(closeErr, catalogShutdownErr) {
		t.Fatalf("explicit Close must not replay historical Run errors: %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected exactly one database close each, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{
		"balance-attempt",
		"catalog-attempt",
		"balance-converged",
		"catalog-converged",
		"transaction",
		"audit",
	}) {
		t.Fatalf("unexpected final cleanup ordering: %v", order)
	}

	repeatedClose := service.Close()
	if !errors.Is(repeatedClose, transactionErr) || !errors.Is(repeatedClose, auditErr) {
		t.Fatalf("expected repeated Close to preserve fresh cleanup errors, got %v", repeatedClose)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}


func TestServiceRepeatedPartialShutdownAttemptsConvergeBeforeDatabaseCleanup(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()
	service.catalogInterval = time.Hour

	firstBalanceErr := errors.New("first balance convergence failure")
	firstCatalogErr := errors.New("first catalog convergence failure")
	secondBalanceErr := errors.New("second balance convergence failure")
	secondCatalogErr := errors.New("second catalog convergence failure")
	transactionErr := errors.New("terminal transaction cleanup failure")
	auditErr := errors.New("terminal audit cleanup failure")
	order := make([]string, 0, 8)

	service.balanceShutdown = func(context.Context) error {
		order = append(order, "balance-attempt")
		return firstBalanceErr
	}
	service.catalogShutdown = func() error {
		order = append(order, "catalog-attempt")
		return firstCatalogErr
	}
	service.catalogStart = func(parent context.Context) (context.Context, error) {
		return service.catalogLifecycle.Start(parent)
	}

	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	firstRunErr := service.Run(ctx)
	if !errors.Is(firstRunErr, context.Canceled) ||
		!errors.Is(firstRunErr, firstBalanceErr) ||
		!errors.Is(firstRunErr, firstCatalogErr) {
		t.Fatalf("expected first partial shutdown to preserve all identities, got %v", firstRunErr)
	}
	if !service.balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		t.Fatal("expected both lifecycles to remain active after first failed convergence")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected database cleanup to remain deferred after first failure: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	service.balanceShutdown = func(context.Context) error {
		order = append(order, "balance-attempt-2")
		return secondBalanceErr
	}
	service.catalogShutdown = func() error {
		order = append(order, "catalog-attempt-2")
		return secondCatalogErr
	}

	secondBalance := service.shutdownBalanceWorker(context.Background())
	secondCatalog := service.shutdownCatalogLifecycle()
	if !errors.Is(secondBalance, secondBalanceErr) {
		t.Fatalf("expected second balance convergence error, got %v", secondBalance)
	}
	if !errors.Is(secondCatalog, secondCatalogErr) {
		t.Fatalf("expected second catalog convergence error, got %v", secondCatalog)
	}
	if !service.balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		t.Fatal("expected both lifecycles to remain active after second failed convergence")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected database cleanup to remain deferred after second failure: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	service.balanceShutdown = func(context.Context) error {
		service.balanceLifecycle.Shutdown(context.Background())
		order = append(order, "balance-converged")
		return nil
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		order = append(order, "catalog-converged")
		return nil
	}

	if err := service.shutdownBalanceWorker(context.Background()); err != nil {
		t.Fatalf("expected final balance convergence, got %v", err)
	}
	if err := service.shutdownCatalogLifecycle(); err != nil {
		t.Fatalf("expected final catalog convergence, got %v", err)
	}
	if service.balanceLifecycle.Running() || service.catalogLifecycle.Running() {
		t.Fatal("expected both lifecycles stopped before database cleanup")
	}

	closeErr := service.Close()
	if !errors.Is(closeErr, transactionErr) || !errors.Is(closeErr, auditErr) {
		t.Fatalf("expected terminal cleanup errors after final convergence, got %v", closeErr)
	}
	if errors.Is(closeErr, firstBalanceErr) || errors.Is(closeErr, firstCatalogErr) ||
		errors.Is(closeErr, secondBalanceErr) || errors.Is(closeErr, secondCatalogErr) {
		t.Fatalf("terminal Close must not replay historical lifecycle errors: %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected exactly one terminal close per database, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{
		"balance-attempt",
		"catalog-attempt",
		"balance-attempt-2",
		"catalog-attempt-2",
		"balance-converged",
		"catalog-converged",
		"transaction",
		"audit",
	}) {
		t.Fatalf("unexpected repeated convergence order: %v", order)
	}

	repeatedCloseErr := service.Close()
	if !errors.Is(repeatedCloseErr, transactionErr) || !errors.Is(repeatedCloseErr, auditErr) {
		t.Fatalf("expected repeated Close to preserve terminal cleanup errors, got %v", repeatedCloseErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected repeated Close not to double-close databases, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
}


func TestServiceMixedShutdownConvergenceDoesNotReplaySuccessfulLifecycle(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogSync, _ = catalog.NewSyncService(
		&provider.Registry{},
		catalog.NewMemoryStore(),
	)
	service.catalogLifecycle = newCatalogWorkerLifecycle()

	catalogFirstErr := errors.New("catalog first convergence failure")
	transactionErr := errors.New("mixed convergence transaction cleanup failure")
	auditErr := errors.New("mixed convergence audit cleanup failure")
	balanceCalls := 0
	catalogCalls := 0
	order := make([]string, 0, 6)

	service.balanceShutdown = func(context.Context) error {
		balanceCalls++
		order = append(order, "balance")
		service.balanceLifecycle.Shutdown(context.Background())
		return nil
	}
	service.catalogShutdown = func() error {
		catalogCalls++
		order = append(order, "catalog")
		if catalogCalls == 1 {
			return catalogFirstErr
		}
		service.catalogLifecycle.Shutdown()
		return nil
	}

	tx := &orderedCloseErrorDB{name: "transaction", order: &order, err: transactionErr}
	audit := &orderedCloseErrorDB{name: "audit", order: &order, err: auditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(tx, audit)
	service.databaseOwnership.transferToService()

	if err := service.balanceLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.catalogLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := service.shutdownBalanceWorker(context.Background()); err != nil {
		t.Fatalf("expected balance to converge successfully, got %v", err)
	}
	firstCatalogErr := service.shutdownCatalogLifecycle()
	if !errors.Is(firstCatalogErr, catalogFirstErr) {
		t.Fatalf("expected first catalog convergence error, got %v", firstCatalogErr)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("expected successful balance convergence to stop the balance lifecycle")
	}
	if !service.catalogLifecycle.Running() {
		t.Fatal("expected failed catalog convergence to leave catalog lifecycle active")
	}
	if tx.closeCount != 0 || audit.closeCount != 0 {
		t.Fatalf("expected database cleanup to remain deferred while catalog is active: tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}

	if err := service.shutdownBalanceWorker(context.Background()); err != nil {
		t.Fatalf("expected already-converged balance lifecycle to be a no-op, got %v", err)
	}
	if err := service.shutdownCatalogLifecycle(); err != nil {
		t.Fatalf("expected second catalog convergence to succeed, got %v", err)
	}
	if balanceCalls != 1 || catalogCalls != 2 {
		t.Fatalf("expected only the still-active catalog to be retried, got balance=%d catalog=%d", balanceCalls, catalogCalls)
	}
	if service.balanceLifecycle.Running() || service.catalogLifecycle.Running() {
		t.Fatal("expected both lifecycles to converge before database cleanup")
	}

	closeErr := service.Close()
	for _, want := range []error{transactionErr, auditErr} {
		if !errors.Is(closeErr, want) {
			t.Fatalf("expected fresh database cleanup error %v, got %v", want, closeErr)
		}
	}
	if errors.Is(closeErr, catalogFirstErr) {
		t.Fatalf("terminal Close must not replay historical catalog convergence error: %v", closeErr)
	}
	if tx.closeCount != 1 || audit.closeCount != 1 {
		t.Fatalf("expected one close per database, got tx=%d audit=%d", tx.closeCount, audit.closeCount)
	}
	if !reflect.DeepEqual(order, []string{"balance", "catalog", "catalog", "transaction", "audit"}) {
		t.Fatalf("unexpected mixed convergence order: %v", order)
	}
}



func TestServiceOwnershipReplacementIsDeferredUntilLifecycleConvergence(t *testing.T) {
	service := newRuntimeTestService(t)

	oldOrder := make([]string, 0, 2)
	oldTx := &orderedCloseErrorDB{name: "old-transaction", order: &oldOrder}
	oldAudit := &orderedCloseErrorDB{name: "old-audit", order: &oldOrder}
	oldOwnership := newRuntimeDatabaseOwnership(oldTx, oldAudit)
	oldOwnership.transferToService()
	service.databaseOwnership = oldOwnership

	newOrder := make([]string, 0, 2)
	newTx := &orderedCloseErrorDB{name: "new-transaction", order: &newOrder}
	newAudit := &orderedCloseErrorDB{name: "new-audit", order: &newOrder}
	newOwnership := newRuntimeDatabaseOwnership(newTx, newAudit)
	newOwnership.transferToService()

	if err := service.balanceLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.replaceDatabaseOwnership(newOwnership); !errors.Is(err, ErrServiceLifecycleActive) {
		t.Fatalf("expected active lifecycle replacement rejection, got %v", err)
	}
	if service.databaseOwnership != oldOwnership {
		t.Fatal("active lifecycle replacement must retain the old ownership generation")
	}
	if oldTx.closeCount != 0 || oldAudit.closeCount != 0 {
		t.Fatalf("old ownership must remain deferred while lifecycle is active: tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
	}
	if newTx.closeCount != 0 || newAudit.closeCount != 0 {
		t.Fatalf("rejected replacement must not touch fresh ownership: tx=%d audit=%d", newTx.closeCount, newAudit.closeCount)
	}

	if err := service.balanceLifecycle.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if service.balanceLifecycle.Running() {
		t.Fatal("expected balance lifecycle to converge before ownership replacement")
	}

	if err := service.replaceDatabaseOwnership(newOwnership); err != nil {
		t.Fatalf("expected replacement after lifecycle convergence, got %v", err)
	}
	if service.databaseOwnership != newOwnership {
		t.Fatal("expected fresh ownership generation to become active after convergence")
	}
	if oldTx.closeCount != 1 || oldAudit.closeCount != 1 {
		t.Fatalf("expected old ownership to close exactly once during replacement: tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
	}
	if !reflect.DeepEqual(oldOrder, []string{"old-transaction", "old-audit"}) {
		t.Fatalf("expected old ownership cleanup order to remain transaction-before-audit, got %v", oldOrder)
	}
	if newTx.closeCount != 0 || newAudit.closeCount != 0 {
		t.Fatalf("fresh ownership must remain open after replacement: tx=%d audit=%d", newTx.closeCount, newAudit.closeCount)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("expected fresh ownership Close to succeed, got %v", err)
	}
	if newTx.closeCount != 1 || newAudit.closeCount != 1 {
		t.Fatalf("expected fresh ownership to close exactly once: tx=%d audit=%d", newTx.closeCount, newAudit.closeCount)
	}
	if !reflect.DeepEqual(newOrder, []string{"new-transaction", "new-audit"}) {
		t.Fatalf("expected fresh ownership cleanup order to remain transaction-before-audit, got %v", newOrder)
	}

	if err := service.replaceDatabaseOwnership(newRuntimeDatabaseOwnership(nil, nil)); err != nil {
		t.Fatalf("expected replacement from already-closed fresh ownership to remain safe, got %v", err)
	}
	if newTx.closeCount != 1 || newAudit.closeCount != 1 {
		t.Fatalf("replacing an already-closed generation must not double-close it: tx=%d audit=%d", newTx.closeCount, newAudit.closeCount)
	}
}


func TestServiceOwnershipReplacementCleanupErrorStaysWithOldGeneration(t *testing.T) {
	tests := []struct {
		name        string
		transaction bool
		audit       bool
	}{
		{name: "transaction cleanup error", transaction: true},
		{name: "audit cleanup error", audit: true},
		{name: "transaction succeeds audit fails", audit: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := newRuntimeTestService(t)

			oldTransactionErr := errors.New("old transaction cleanup error")
			oldAuditErr := errors.New("old audit cleanup error")
			oldOrder := make([]string, 0, 2)
			oldTx := &orderedCloseErrorDB{name: "old-transaction", order: &oldOrder}
			oldAudit := &orderedCloseErrorDB{name: "old-audit", order: &oldOrder}
			if tc.transaction {
				oldTx.err = oldTransactionErr
			}
			if tc.audit {
				oldAudit.err = oldAuditErr
			}
			oldOwnership := newRuntimeDatabaseOwnership(oldTx, oldAudit)
			oldOwnership.transferToService()
			service.databaseOwnership = oldOwnership

			freshTransactionErr := errors.New("fresh transaction cleanup error")
			freshAuditErr := errors.New("fresh audit cleanup error")
			freshOrder := make([]string, 0, 2)
			freshTx := &orderedCloseErrorDB{name: "fresh-transaction", order: &freshOrder, err: freshTransactionErr}
			freshAudit := &orderedCloseErrorDB{name: "fresh-audit", order: &freshOrder, err: freshAuditErr}
			freshOwnership := newRuntimeDatabaseOwnership(freshTx, freshAudit)
			freshOwnership.transferToService()

			firstErr := service.replaceDatabaseOwnership(freshOwnership)
			if firstErr == nil {
				t.Fatal("expected replacement to fail when old ownership cleanup returns an error")
			}
			if service.databaseOwnership != oldOwnership {
				t.Fatal("failed replacement must retain the old ownership generation")
			}
			if tc.transaction && !errors.Is(firstErr, oldTransactionErr) {
				t.Fatalf("expected old transaction cleanup error attribution, got %v", firstErr)
			}
			if tc.audit && !errors.Is(firstErr, oldAuditErr) {
				t.Fatalf("expected old audit cleanup error attribution, got %v", firstErr)
			}
			if errors.Is(firstErr, freshTransactionErr) || errors.Is(firstErr, freshAuditErr) {
				t.Fatalf("fresh generation errors must not leak into failed replacement: %v", firstErr)
			}
			if oldTx.closeCount != 1 || oldAudit.closeCount != 1 {
				t.Fatalf("old generation must be closed exactly once after terminal cleanup attempt: tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
			}
			if !reflect.DeepEqual(oldOrder, []string{"old-transaction", "old-audit"}) {
				t.Fatalf("expected transaction-before-audit cleanup ordering, got %v", oldOrder)
			}
			if freshTx.closeCount != 0 || freshAudit.closeCount != 0 {
				t.Fatalf("fresh generation must remain untouched after failed replacement: tx=%d audit=%d", freshTx.closeCount, freshAudit.closeCount)
			}

			secondErr := service.replaceDatabaseOwnership(freshOwnership)
			if secondErr != nil {
				t.Fatalf("expected retry after old generation became terminal to install fresh generation, got %v", secondErr)
			}
			if service.databaseOwnership != freshOwnership {
				t.Fatal("expected fresh ownership generation to become active after retry")
			}
			if oldTx.closeCount != 1 || oldAudit.closeCount != 1 {
				t.Fatalf("retry must not double-close old generation: tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
			}
			if freshTx.closeCount != 0 || freshAudit.closeCount != 0 {
				t.Fatalf("fresh generation must start clean and remain open after installation: tx=%d audit=%d", freshTx.closeCount, freshAudit.closeCount)
			}

			thirdErr := service.replaceDatabaseOwnership(freshOwnership)
			if thirdErr != nil {
				t.Fatalf("repeated replacement of the active fresh generation must be idempotent, got %v", thirdErr)
			}
			if oldTx.closeCount != 1 || oldAudit.closeCount != 1 {
				t.Fatalf("repeated replacement must not double-close old generation: tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
			}

			freshCloseErr := service.Close()
			if !errors.Is(freshCloseErr, freshTransactionErr) || !errors.Is(freshCloseErr, freshAuditErr) {
				t.Fatalf("terminal Close must report only fresh generation cleanup errors, got %v", freshCloseErr)
			}
			if errors.Is(freshCloseErr, oldTransactionErr) || errors.Is(freshCloseErr, oldAuditErr) {
				t.Fatalf("terminal fresh Close must not replay old-generation cleanup errors: %v", freshCloseErr)
			}
			if freshTx.closeCount != 1 || freshAudit.closeCount != 1 {
				t.Fatalf("fresh generation must close exactly once at terminal Close: tx=%d audit=%d", freshTx.closeCount, freshAudit.closeCount)
			}
			if !reflect.DeepEqual(freshOrder, []string{"fresh-transaction", "fresh-audit"}) {
				t.Fatalf("expected fresh transaction-before-audit cleanup ordering, got %v", freshOrder)
			}

			repeatedCloseErr := service.Close()
			if !errors.Is(repeatedCloseErr, freshTransactionErr) || !errors.Is(repeatedCloseErr, freshAuditErr) {
				t.Fatalf("repeated fresh Close must preserve fresh cleanup errors, got %v", repeatedCloseErr)
			}
			if errors.Is(repeatedCloseErr, oldTransactionErr) || errors.Is(repeatedCloseErr, oldAuditErr) {
				t.Fatalf("repeated fresh Close must not replay old-generation errors: %v", repeatedCloseErr)
			}
			if freshTx.closeCount != 1 || freshAudit.closeCount != 1 {
				t.Fatalf("repeated fresh Close must not double-close fresh generation: tx=%d audit=%d", freshTx.closeCount, freshAudit.closeCount)
			}
		})
	}
}

func TestServiceOwnershipReplacementConcurrentLifecycleActivityCannotInstallFreshGeneration(t *testing.T) {
	service := newRuntimeTestService(t)
	service.catalogLifecycle = newCatalogWorkerLifecycle()

	oldTx := &closeErrorDB{}
	oldAudit := &closeErrorDB{}
	oldOwnership := newRuntimeDatabaseOwnership(oldTx, oldAudit)
	oldOwnership.transferToService()
	service.databaseOwnership = oldOwnership

	freshTx := &closeErrorDB{}
	freshAudit := &closeErrorDB{}
	freshOwnership := newRuntimeDatabaseOwnership(freshTx, freshAudit)
	freshOwnership.transferToService()

	if err := service.balanceLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.catalogLifecycle.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	const attempts = 64
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- service.replaceDatabaseOwnership(freshOwnership)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if !errors.Is(err, ErrServiceLifecycleActive) {
			t.Fatalf("concurrent replacement during active lifecycle must be rejected, got %v", err)
		}
	}
	if service.databaseOwnership != oldOwnership {
		t.Fatal("concurrent active-lifecycle replacement must not install fresh ownership")
	}
	if oldTx.closeCount != 0 || oldAudit.closeCount != 0 {
		t.Fatalf("old generation must remain untouched while lifecycle activity is present: tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
	}
	if freshTx.closeCount != 0 || freshAudit.closeCount != 0 {
		t.Fatalf("fresh generation must remain untouched while replacement is rejected: tx=%d audit=%d", freshTx.closeCount, freshAudit.closeCount)
	}

	service.catalogLifecycle.Shutdown()
	if err := service.balanceLifecycle.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if service.balanceLifecycle.Running() || service.catalogLifecycle.Running() {
		t.Fatal("expected both lifecycle owners to converge before replacement retry")
	}

	if err := service.replaceDatabaseOwnership(freshOwnership); err != nil {
		t.Fatalf("expected replacement after concurrent lifecycle convergence, got %v", err)
	}
	if service.databaseOwnership != freshOwnership {
		t.Fatal("expected fresh generation to become active after lifecycle convergence")
	}
	if oldTx.closeCount != 1 || oldAudit.closeCount != 1 {
		t.Fatalf("expected old generation to close exactly once after convergence: tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
	}
	if freshTx.closeCount != 0 || freshAudit.closeCount != 0 {
		t.Fatalf("fresh generation must remain open after installation: tx=%d audit=%d", freshTx.closeCount, freshAudit.closeCount)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("expected terminal fresh Close to succeed, got %v", err)
	}
	if freshTx.closeCount != 1 || freshAudit.closeCount != 1 {
		t.Fatalf("expected fresh generation to close exactly once, got tx=%d audit=%d", freshTx.closeCount, freshAudit.closeCount)
	}
}

func TestServiceFreshOwnershipReplacementDoesNotReplayStaleCleanupErrors(t *testing.T) {
	registry := provider.NewRegistry()
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
	service.catalogInterval = time.Hour
	service.catalogLifecycle = newCatalogWorkerLifecycle()

	oldTransactionErr := errors.New("stale transaction cleanup failure")
	oldAuditErr := errors.New("stale audit cleanup failure")
	oldOrder := make([]string, 0, 2)
	oldTx := &orderedCloseErrorDB{name: "old-transaction", order: &oldOrder, err: oldTransactionErr}
	oldAudit := &orderedCloseErrorDB{name: "old-audit", order: &oldOrder, err: oldAuditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(oldTx, oldAudit)
	service.databaseOwnership.transferToService()

	oldCloseErr := service.Close()
	if !errors.Is(oldCloseErr, oldTransactionErr) || !errors.Is(oldCloseErr, oldAuditErr) {
		t.Fatalf("expected old ownership cleanup errors, got %v", oldCloseErr)
	}
	if oldTx.closeCount != 1 || oldAudit.closeCount != 1 {
		t.Fatalf("expected old ownership generation to close exactly once, got tx=%d audit=%d", oldTx.closeCount, oldAudit.closeCount)
	}

	newTransactionErr := errors.New("fresh transaction cleanup failure")
	newAuditErr := errors.New("fresh audit cleanup failure")
	newOrder := make([]string, 0, 2)
	newTx := &orderedCloseErrorDB{name: "new-transaction", order: &newOrder, err: newTransactionErr}
	newAudit := &orderedCloseErrorDB{name: "new-audit", order: &newOrder, err: newAuditErr}
	service.databaseOwnership = newRuntimeDatabaseOwnership(newTx, newAudit)
	service.databaseOwnership.transferToService()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.balanceStart = func(context.Context) error {
		return service.balanceLifecycle.Start(context.Background())
	}
	service.catalogStart = func(context.Context) (context.Context, error) {
		return service.catalogLifecycle.Start(context.Background())
	}

	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	deadline := time.After(time.Second)
	for !service.balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		select {
		case <-deadline:
			t.Fatal("fresh ownership Run did not start both lifecycles")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()

	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected fresh ownership Run cancellation, got %v", runErr)
		}
		if errors.Is(runErr, oldTransactionErr) || errors.Is(runErr, oldAuditErr) {
			t.Fatalf("fresh ownership Run replayed stale cleanup errors: %v", runErr)
		}
		if !errors.Is(runErr, newTransactionErr) || !errors.Is(runErr, newAuditErr) {
			t.Fatalf("expected fresh ownership cleanup errors, got %v", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("fresh ownership Run did not shut down")
	}

	if newTx.closeCount != 1 || newAudit.closeCount != 1 {
		t.Fatalf("expected fresh ownership generation to close exactly once, got tx=%d audit=%d", newTx.closeCount, newAudit.closeCount)
	}
	repeatedCloseErr := service.Close()
	if !errors.Is(repeatedCloseErr, newTransactionErr) || !errors.Is(repeatedCloseErr, newAuditErr) {
		t.Fatalf("expected repeated Close to preserve only fresh ownership cleanup errors, got %v", repeatedCloseErr)
	}
	if oldTx.closeCount != 1 || oldAudit.closeCount != 1 || newTx.closeCount != 1 || newAudit.closeCount != 1 {
		t.Fatal("ownership generations must remain single-shot after repeated Close")
	}
}
