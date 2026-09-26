package runtime

import (
	"errors"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestOpenTransactionStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	cfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	store, db, err := openTransactionStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := "runtime_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }

	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_provider_transactions.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	pgStore, ok := store.(*routing.PostgresTransactionStore)
	if !ok {
		t.Fatalf("expected PostgreSQL transaction store, got %T", store)
	}
	states, err := pgStore.AllContextE(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 0 {
		t.Fatalf("expected fresh integration table, got %d transactions", len(states))
	}
}

func TestOpenAuditStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	ctx := context.Background()
	cfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	auditStore, db, err := openAuditStore(ctx, cfg, nil)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	db.SetMaxOpenConns(1)
	schema := "runtime_audit_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatal(err) }
	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_provider_transactions.sql"))
	if err != nil { t.Fatal(err) }
	if _, err := db.ExecContext(ctx, string(migration)); err != nil { t.Fatal(err) }
	pgStore, ok := auditStore.(*routing.PostgresTransactionAuditStore)
	if !ok { t.Fatalf("expected PostgreSQL audit store, got %T", auditStore) }
	event := routing.TransactionAuditEvent{ReferenceID:"runtime-audit-1",Action:"PURCHASE_RESULT",Previous:"pending",Next:"success",ProviderName:"mock",Message:"success",CreatedAt:time.Now().UTC()}
	if err := pgStore.AppendContext(ctx, event); err != nil { t.Fatal(err) }
	reloaded, err := pgStore.AllContextE(ctx, event.ReferenceID)
	if err != nil { t.Fatal(err) }
	if len(reloaded) != 1 || reloaded[0].ReferenceID != event.ReferenceID || reloaded[0].Action != event.Action || reloaded[0].Next != event.Next { t.Fatalf("unexpected durable audit events: %#v", reloaded) }
}


func TestServiceCloseClosesSharedAndDedicatedPostgresOwnership(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	ctx := context.Background()

	sharedCfg := Config{TransactionStoreDriver: "postgres", AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, sharedDB, err := openTransactionStore(ctx, sharedCfg)
	if err != nil { t.Fatal(err) }
	if _, _, err := openAuditStore(ctx, sharedCfg, sharedDB); err != nil { t.Fatal(err) }
	sharedService := &Service{databaseOwnership: newRuntimeDatabaseOwnership(sharedDB, sharedDB)}
	sharedService.databaseOwnership.transferToService()
	if err := sharedService.Close(); err != nil { t.Fatal(err) }
	if err := sharedDB.PingContext(ctx); err == nil { t.Fatal("expected shared PostgreSQL handle to be closed by Service.Close") }
	if err := sharedService.Close(); err != nil { t.Fatal(err) }

	dedicatedCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, dedicatedDB, err := openAuditStore(ctx, dedicatedCfg, nil)
	if err != nil { t.Fatal(err) }
	dedicatedService := &Service{databaseOwnership: newRuntimeDatabaseOwnership(nil, dedicatedDB)}
	dedicatedService.databaseOwnership.transferToService()
	if err := dedicatedService.Close(); err != nil { t.Fatal(err) }
	if err := dedicatedDB.PingContext(ctx); err == nil { t.Fatal("expected dedicated PostgreSQL audit handle to be closed by Service.Close") }
	if err := dedicatedService.Close(); err != nil { t.Fatal(err) }
}

func TestCloseRuntimeDatabasesClosesSharedAndDedicatedHandles(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	ctx := context.Background()

	sharedCfg := Config{TransactionStoreDriver: "postgres", AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, sharedDB, err := openTransactionStore(ctx, sharedCfg)
	if err != nil { t.Fatal(err) }
	if _, _, err := openAuditStore(ctx, sharedCfg, sharedDB); err != nil { t.Fatal(err) }
	sharedOwnership := newRuntimeDatabaseOwnership(sharedDB, sharedDB)
	sharedOwnership.transferToService()
	if err := sharedOwnership.cleanupBeforeTransfer(); err != nil { t.Fatal(err) }
	if err := sharedDB.PingContext(ctx); err != nil { t.Fatalf("initialization guard closed transferred shared handle: %v", err) }
	if err := sharedOwnership.closeOwned(); err != nil { t.Fatal(err) }
	if err := sharedDB.PingContext(ctx); err == nil { t.Fatal("expected shared PostgreSQL handle to be closed") }
	if err := sharedOwnership.closeOwned(); err != nil { t.Fatal(err) }

	dedicatedCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, dedicatedDB, err := openAuditStore(ctx, dedicatedCfg, nil)
	if err != nil { t.Fatal(err) }
	dedicatedOwnership := newRuntimeDatabaseOwnership(nil, dedicatedDB)
	dedicatedOwnership.transferToService()
	if err := dedicatedOwnership.cleanupBeforeTransfer(); err != nil { t.Fatal(err) }
	if err := dedicatedDB.PingContext(ctx); err != nil { t.Fatalf("initialization guard closed transferred dedicated audit handle: %v", err) }
	if err := dedicatedOwnership.closeOwned(); err != nil { t.Fatal(err) }
	if err := dedicatedDB.PingContext(ctx); err == nil { t.Fatal("expected dedicated PostgreSQL audit handle to be closed") }
	if err := dedicatedOwnership.closeOwned(); err != nil { t.Fatal(err) }
}


func TestPostgresRuntimeReopenPreservesSharedTransactionAndAuditOwnership(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}
	ctx := context.Background()
	cfg := Config{TransactionStoreDriver: "postgres", AuditStoreDriver: "postgres", PostgresDSN: dsn}

	transactionStore, firstDB, err := openTransactionStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	firstDB.SetMaxOpenConns(1)
	schema := "runtime_reopen_shared_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := firstDB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}
	defer func() {
		cleanupDB, err := sql.Open("pgx", dsn)
		if err == nil {
			defer cleanupDB.Close()
			_, _ = cleanupDB.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		}
	}()
	if _, err := firstDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_provider_transactions.sql"))
	if err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}
	if _, err := firstDB.ExecContext(ctx, string(migration)); err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}

	auditStore, auditDB, err := openAuditStore(ctx, cfg, firstDB)
	if err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}
	if auditDB != nil {
		t.Fatalf("shared audit store must not acquire a second database handle, got %T", auditDB)
	}

	reference := "runtime-reopen-shared-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	state := routing.TransactionState{
		Request: routing.PurchaseRequest{ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: reference, Amount: 10000},
		Version: 2,
		Execution: routing.PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: reference,
				CustomerNo: "087800001232",
				ProductCode: "xld10",
				Status: provider.StatusSuccess,
				ProviderCode: "00",
				Message: "ok",
				Price: 10000,
			},
		},
	}
	pgStore := transactionStore.(*routing.PostgresTransactionStore)
	if err := pgStore.PutContext(ctx, routing.TransactionState{
		Request: state.Request,
		Execution: routing.PurchaseExecution{
			ProviderName: state.Execution.ProviderName,
			Result: provider.PurchaseResult{
				ReferenceID: reference,
				CustomerNo: state.Request.CustomerNo,
				ProductCode: state.Request.ProductCode,
				Status: provider.StatusPending,
				ProviderCode: "00",
				Message: "pending",
			},
		},
	}); err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}
	if err := pgStore.PutIfCurrentContext(ctx, reference, routing.TransactionState{
		Request: state.Request,
		Version: 1,
		Execution: routing.PurchaseExecution{
			ProviderName: state.Execution.ProviderName,
			Result: provider.PurchaseResult{
				ReferenceID: reference,
				CustomerNo: state.Request.CustomerNo,
				ProductCode: state.Request.ProductCode,
				Status: provider.StatusPending,
				ProviderCode: "00",
				Message: "pending",
			},
		},
	}, state); err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}
	event := routing.TransactionAuditEvent{
		ReferenceID: reference,
		Action: "REOPEN_CHECK",
		Previous: "pending",
		Next: "success",
		ProviderName: "mock",
		Message: "durable before reopen",
		CreatedAt: time.Now().UTC(),
	}
	if err := auditStore.(*routing.PostgresTransactionAuditStore).AppendContext(ctx, event); err != nil {
		_ = firstDB.Close()
		t.Fatal(err)
	}

	ownership := newRuntimeDatabaseOwnership(firstDB, firstDB)
	ownership.transferToService()
	if err := ownership.closeOwned(); err != nil {
		t.Fatal(err)
	}
	if err := firstDB.PingContext(ctx); err == nil {
		t.Fatal("expected first shared database handle to be closed before reopen")
	}

	reopenedStore, secondDB, err := openTransactionStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	secondDB.SetMaxOpenConns(1)
	if _, err := secondDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		_ = secondDB.Close()
		t.Fatal(err)
	}
	reopenedAuditStore, reopenedAuditDB, err := openAuditStore(ctx, cfg, secondDB)
	if err != nil {
		_ = secondDB.Close()
		t.Fatal(err)
	}
	if reopenedAuditDB != nil {
		_ = secondDB.Close()
		t.Fatalf("reopened shared audit store must reuse the new transaction database handle, got %T", reopenedAuditDB)
	}
	defer secondDB.Close()

	reloaded, ok, err := reopenedStore.(*routing.PostgresTransactionStore).GetContextE(ctx, reference)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected transaction after reopen")
	}
	if reloaded.Execution.Result.Status != provider.StatusSuccess || reloaded.Version != state.Version {
		t.Fatalf("unexpected transaction after reopen: %#v", reloaded)
	}
	events, err := reopenedAuditStore.(*routing.PostgresTransactionAuditStore).AllContextE(ctx, reference)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Action != event.Action || events[0].Next != event.Next {
		t.Fatalf("unexpected audit history after reopen: %#v", events)
	}
	if secondDB == firstDB {
		t.Fatal("reopen must acquire a fresh database handle")
	}
}

func TestPostgresRuntimeReopenPreservesDedicatedAuditOwnership(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}
	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}

	transactionStore, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	transactionDB.SetMaxOpenConns(1)
	schema := "runtime_reopen_dedicated_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := transactionDB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	defer func() {
		cleanupDB, err := sql.Open("pgx", dsn)
		if err == nil {
			defer cleanupDB.Close()
			_, _ = cleanupDB.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
		}
	}()
	if _, err := transactionDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_provider_transactions.sql"))
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if _, err := transactionDB.ExecContext(ctx, string(migration)); err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}

	auditStore, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	auditDB.SetMaxOpenConns(1)
	if _, err := auditDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		_ = transactionDB.Close()
		_ = auditDB.Close()
		t.Fatal(err)
	}

	reference := "runtime-reopen-dedicated-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	state := routing.TransactionState{
		Request: routing.PurchaseRequest{ProductCode: "xld10", CustomerNo: "087800001232", ReferenceID: reference, Amount: 10000},
		Execution: routing.PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: reference,
				CustomerNo: "087800001232",
				ProductCode: "xld10",
				Status: provider.StatusSuccess,
				ProviderCode: "00",
				Message: "ok",
				Price: 10000,
			},
		},
	}
	pgStore := transactionStore.(*routing.PostgresTransactionStore)
	pending := state
	pending.Execution.Result.Status = provider.StatusPending
	pending.Version = 1
	pending.Execution.Result.Message = "pending"
	if err := pgStore.PutContext(ctx, pending); err != nil {
		_ = transactionDB.Close()
		_ = auditDB.Close()
		t.Fatal(err)
	}
	if err := pgStore.PutIfCurrentContext(ctx, reference, pending, state); err != nil {
		_ = transactionDB.Close()
		_ = auditDB.Close()
		t.Fatal(err)
	}
	event := routing.TransactionAuditEvent{
		ReferenceID: reference,
		Action: "REOPEN_DEDICATED_CHECK",
		Previous: "pending",
		Next: "success",
		ProviderName: "mock",
		Message: "durable before reopen",
		CreatedAt: time.Now().UTC(),
	}
	if err := auditStore.(*routing.PostgresTransactionAuditStore).AppendContext(ctx, event); err != nil {
		_ = transactionDB.Close()
		_ = auditDB.Close()
		t.Fatal(err)
	}

	ownership := newRuntimeDatabaseOwnership(transactionDB, auditDB)
	ownership.transferToService()
	if err := ownership.closeOwned(); err != nil {
		t.Fatal(err)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction database handle to be closed before reopen")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected dedicated audit database handle to be closed before reopen")
	}

	reopenedTransactionStore, reopenedTransactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	reopenedTransactionDB.SetMaxOpenConns(1)
	if _, err := reopenedTransactionDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		_ = reopenedTransactionDB.Close()
		t.Fatal(err)
	}
	reopenedAuditStore, reopenedAuditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = reopenedTransactionDB.Close()
		t.Fatal(err)
	}
	reopenedAuditDB.SetMaxOpenConns(1)
	if _, err := reopenedAuditDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		_ = reopenedTransactionDB.Close()
		_ = reopenedAuditDB.Close()
		t.Fatal(err)
	}
	defer reopenedTransactionDB.Close()
	defer reopenedAuditDB.Close()

	reloaded, ok, err := reopenedTransactionStore.(*routing.PostgresTransactionStore).GetContextE(ctx, reference)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected transaction after dedicated reopen")
	}
	if reloaded.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("unexpected transaction after dedicated reopen: %#v", reloaded)
	}
	events, err := reopenedAuditStore.(*routing.PostgresTransactionAuditStore).AllContextE(ctx, reference)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Action != event.Action || events[0].Next != event.Next {
		t.Fatalf("unexpected dedicated audit history after reopen: %#v", events)
	}
	if reopenedTransactionDB == reopenedAuditDB {
		t.Fatal("dedicated transaction and audit stores must acquire independent database handles")
	}
}


func TestNewFromEnvironmentContextRollsBackSharedPostgresOwnershipBeforeTransfer(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", dsn)

	expected := errors.New("injected startup failure after database acquisition")
	var captured *runtimeDatabaseOwnership
	runtimeInitializationFailureHook = func(stage string, ownership *runtimeDatabaseOwnership) error {
		if stage != "after-database-acquisition" { t.Fatalf("unexpected initialization hook stage: %q", stage) }
		captured = ownership
		return expected
	}
	defer func() { runtimeInitializationFailureHook = nil }()

	_, err := NewFromEnvironmentContext(context.Background(), nil)
	if !errors.Is(err, expected) { t.Fatalf("expected injected startup error, got %v", err) }
	if captured == nil { t.Fatal("expected startup hook to capture database ownership") }
	if captured.transferred() { t.Fatal("database ownership must not transfer after startup failure") }
	if !captured.closed { t.Fatal("expected acquired shared database ownership to be closed during startup rollback") }
	if databaseCloserIsNil(captured.transactionDB) { t.Fatal("expected shared transaction PostgreSQL database handle") }
	if !databaseCloserIsNil(captured.auditDB) { t.Fatal("shared audit store must reuse transaction database handle without owning a second database") }
	if err := captured.closeOwned(); err != nil { t.Fatalf("repeated ownership cleanup failed: %v", err) }
}

func TestNewFromEnvironmentContextRollsBackDedicatedAuditPostgresOwnershipBeforeTransfer(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "json")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", dsn)

	expected := errors.New("injected startup failure after database acquisition")
	var captured *runtimeDatabaseOwnership
	runtimeInitializationFailureHook = func(stage string, ownership *runtimeDatabaseOwnership) error {
		if stage != "after-database-acquisition" { t.Fatalf("unexpected initialization hook stage: %q", stage) }
		captured = ownership
		return expected
	}
	defer func() { runtimeInitializationFailureHook = nil }()

	_, err := NewFromEnvironmentContext(context.Background(), nil)
	if !errors.Is(err, expected) { t.Fatalf("expected injected startup error, got %v", err) }
	if captured == nil { t.Fatal("expected startup hook to capture database ownership") }
	if captured.transferred() { t.Fatal("database ownership must not transfer after startup failure") }
	if !captured.closed { t.Fatal("expected dedicated audit database ownership to be closed during startup rollback") }
	if !databaseCloserIsNil(captured.transactionDB) || databaseCloserIsNil(captured.auditDB) { t.Fatalf("expected only dedicated audit database ownership, got tx=%T audit=%T", captured.transactionDB, captured.auditDB) }
	if err := captured.closeOwned(); err != nil { t.Fatalf("repeated ownership cleanup failed: %v", err) }
}


func TestNewFromEnvironmentContextStartupFailureMatrixClosesDedicatedAuditOwnership(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	stages := []string{"after-provider-state-store", "after-router", "after-purchase-service"}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
			t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
			t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "json")
			t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
			t.Setenv("DESKAPROVIDER_POSTGRES_DSN", dsn)
			t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", filepath.Join(t.TempDir(), "provider-state.json"))
			expected := errors.New("injected startup matrix failure")
			var captured *runtimeDatabaseOwnership
			runtimeInitializationFailureHook = func(got string, ownership *runtimeDatabaseOwnership) error {
				if got != stage { return nil }
				captured = ownership
				return expected
			}
			defer func() { runtimeInitializationFailureHook = nil }()
			_, err := NewFromEnvironmentContext(context.Background(), nil)
			if !errors.Is(err, expected) { t.Fatalf("expected injected startup error, got %v", err) }
			if captured == nil { t.Fatal("expected captured ownership") }
			if captured.transferred() { t.Fatal("ownership transferred after startup failure") }
			if databaseCloserIsNil(captured.auditDB) { t.Fatal("expected dedicated audit database ownership") }
			if !captured.closed { t.Fatal("expected startup rollback to close dedicated audit database") }
			if err := captured.closeOwned(); err != nil { t.Fatalf("repeated cleanup failed: %v", err) }
		})
	}
}


type runtimeCleanupErrorDB struct {
	delegate databaseCloser
	err      error
	closeCount int
}

func (db *runtimeCleanupErrorDB) Close() error {
	db.closeCount++
	if db.delegate != nil {
		if err := db.delegate.Close(); err != nil {
			return errors.Join(db.err, err)
		}
	}
	return db.err
}

func TestNewFromEnvironmentContextPreservesInitializationErrorWhenCleanupAlsoFails(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "json")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", dsn)

	stages := []string{"after-provider-state-store", "after-router", "after-purchase-service", "before-ownership-transfer"}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", filepath.Join(t.TempDir(), "provider-state.json"))
			expected := errors.New("injected constructor initialization failure")
			cleanupErr := errors.New("injected constructor cleanup failure")
			var captured *runtimeCleanupErrorDB
			runtimeInitializationFailureHook = func(got string, ownership *runtimeDatabaseOwnership) error {
				if got != stage { return nil }
				if databaseCloserIsNil(ownership.auditDB) { t.Fatal("expected dedicated audit database ownership") }
				captured = &runtimeCleanupErrorDB{delegate: ownership.auditDB, err: cleanupErr}
				ownership.auditDB = captured
				return expected
			}
			defer func() { runtimeInitializationFailureHook = nil }()

			_, err := NewFromEnvironmentContext(context.Background(), nil)
			if !errors.Is(err, expected) { t.Fatalf("expected primary initialization error, got %v", err) }
			if !errors.Is(err, cleanupErr) { t.Fatalf("expected cleanup error to remain discoverable, got %v", err) }
			if captured == nil || captured.closeCount != 1 { t.Fatalf("expected wrapped audit resource to close exactly once, got %#v", captured) }
			if !strings.Contains(err.Error(), "close audit database") { t.Fatalf("expected audit cleanup context in error, got %v", err) }
		})
	}
}


func TestNewFromEnvironmentContextPreservesInitializationErrorWhenSharedCleanupAlsoFails(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", dsn)

	expected := errors.New("injected shared constructor initialization failure")
	cleanupErr := errors.New("injected shared constructor cleanup failure")
	var captured *runtimeCleanupErrorDB
	runtimeInitializationFailureHook = func(stage string, ownership *runtimeDatabaseOwnership) error {
		if stage != "after-database-acquisition" { return nil }
		if databaseCloserIsNil(ownership.transactionDB) { t.Fatal("expected shared transaction database ownership") }
		if !databaseCloserIsNil(ownership.auditDB) { t.Fatal("shared audit ownership must be nil") }
		captured = &runtimeCleanupErrorDB{delegate: ownership.transactionDB, err: cleanupErr}
		ownership.transactionDB = captured
		return expected
	}
	defer func() { runtimeInitializationFailureHook = nil }()

	_, err := NewFromEnvironmentContext(context.Background(), nil)
	if !errors.Is(err, expected) { t.Fatalf("expected primary initialization error, got %v", err) }
	if !errors.Is(err, cleanupErr) { t.Fatalf("expected shared cleanup error to remain discoverable, got %v", err) }
	if captured == nil || captured.closeCount != 1 { t.Fatalf("expected shared resource to close exactly once, got %#v", captured) }
	if !strings.Contains(err.Error(), "close transaction database") { t.Fatalf("expected transaction cleanup context in error, got %v", err) }
}

func TestNewFromEnvironmentContextSharedOwnershipCleanupIsSingleShotAfterInitializationFailure(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", dsn)

	expected := errors.New("injected shared ownership failure")
	var captured *runtimeDatabaseOwnership
	runtimeInitializationFailureHook = func(stage string, ownership *runtimeDatabaseOwnership) error {
		if stage != "after-database-acquisition" { return nil }
		captured = ownership
		if databaseCloserIsNil(ownership.transactionDB) { t.Fatal("expected shared transaction database ownership") }
		if !databaseCloserIsNil(ownership.auditDB) { t.Fatal("shared audit ownership must be nil") }
		return expected
	}
	defer func() { runtimeInitializationFailureHook = nil }()

	_, err := NewFromEnvironmentContext(context.Background(), nil)
	if !errors.Is(err, expected) { t.Fatalf("expected injected initialization error, got %v", err) }
	if captured == nil { t.Fatal("expected captured shared ownership") }
	if !captured.closed { t.Fatal("expected shared ownership cleanup") }
	if err := captured.closeOwned(); err != nil { t.Fatalf("repeated shared cleanup failed: %v", err) }
}

func TestNewFromEnvironmentContextCancellationBeforeOwnershipTransferClosesDedicatedAuditOwnership(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" { t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured") }
	t.Setenv("DIGIFLAZZ_USERNAME", "test-user")
	t.Setenv("DIGIFLAZZ_API_KEY", "test-key")
	t.Setenv("DESKAPROVIDER_TRANSACTION_STORE_DRIVER", "json")
	t.Setenv("DESKAPROVIDER_AUDIT_STORE_DRIVER", "postgres")
	t.Setenv("DESKAPROVIDER_POSTGRES_DSN", dsn)
	t.Setenv("DESKAPROVIDER_PROVIDER_STATE_STORE_PATH", filepath.Join(t.TempDir(), "provider-state.json"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var captured *runtimeDatabaseOwnership
	runtimeInitializationFailureHook = func(stage string, ownership *runtimeDatabaseOwnership) error {
		if stage != "before-ownership-transfer" { return nil }
		captured = ownership
		cancel()
		return nil
	}
	defer func() { runtimeInitializationFailureHook = nil }()
	_, err := NewFromEnvironmentContext(ctx, nil)
	if !errors.Is(err, context.Canceled) { t.Fatalf("expected context.Canceled, got %v", err) }
	if captured == nil { t.Fatal("expected captured ownership") }
	if captured.transferred() { t.Fatal("ownership transferred after canceled initialization") }
	if captured.closed == false { t.Fatal("expected startup rollback to close dedicated audit database") }
	if err := captured.closeOwned(); err != nil { t.Fatalf("repeated cleanup failed: %v", err) }
}
