package runtime

import (
	"reflect"
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
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func TestMain(m *testing.M) {
	os.Setenv("DESKAPROVIDER_POSTGRES_SCHEMA_MODE", "")
	os.Exit(m.Run())
}

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
	paymentMigration, err := os.ReadFile(filepath.Join("..", "migrations", "003_payment_transactions.sql"))
	if err != nil { t.Fatal(err) }
	if _, err := db.ExecContext(ctx, string(paymentMigration)); err != nil { t.Fatal(err) }

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
	name      string
	order     *[]string
}

func (db *runtimeCleanupErrorDB) Close() error {
	db.closeCount++
	if db.order != nil {
		*db.order = append(*db.order, db.name)
	}
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


func TestServiceRunShutdownPreservesPrimaryLifecycleAndSharedPostgresCleanupErrors(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	cfg := Config{TransactionStoreDriver: "postgres", AuditStoreDriver: "postgres", PostgresDSN: dsn}
	transactionStore, transactionDB, err := openTransactionStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if transactionDB == nil {
		t.Fatal("expected shared PostgreSQL transaction database handle")
	}
	transactionDB.SetMaxOpenConns(1)

	auditStore, auditDB, err := openAuditStore(ctx, cfg, transactionDB)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if auditStore == nil {
		_ = transactionDB.Close()
		t.Fatal("expected shared PostgreSQL audit store")
	}
	if auditDB != nil {
		_ = transactionDB.Close()
		t.Fatalf("shared PostgreSQL audit store must not own a second database handle, got %T", auditDB)
	}

	cleanupErr := errors.New("injected shared shutdown cleanup failure")
	wrappedDB := &runtimeCleanupErrorDB{delegate: transactionDB, err: cleanupErr}
	ownership := newRuntimeDatabaseOwnership(wrappedDB, nil)
	ownership.transferToService()

	registry := provider.NewRegistry()
	balanceProvider := &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}
	if err := registry.Register("mock", balanceProvider); err != nil {
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
	service := &Service{
		syncService:      syncService,
		balanceLifecycle: balanceLifecycle,
		databaseOwnership: ownership,
		interval:         time.Hour,
	}

	runCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		result <- service.Run(runCtx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !balanceLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for balance worker to start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected primary lifecycle cancellation error, got %v", err)
		}
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("expected shared database cleanup error to remain discoverable, got %v", err)
		}
		if !strings.Contains(err.Error(), "close transaction database") {
			t.Fatalf("expected transaction database cleanup context, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for service shutdown")
	}

	if wrappedDB.closeCount != 1 {
		t.Fatalf("expected shared database to close exactly once, got %d", wrappedDB.closeCount)
	}
	if ownership.transferred() == false {
		t.Fatal("expected database ownership to remain transferred during service shutdown")
	}
	if err := service.Close(); !errors.Is(err, cleanupErr) {
		t.Fatalf("repeated service close must preserve the stored cleanup error without closing again, got %v", err)
	}
	if wrappedDB.closeCount != 1 {
		t.Fatalf("expected repeated service close not to close shared database again, got %d", wrappedDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected shared PostgreSQL database handle to be closed after service shutdown")
	}
	_ = transactionStore
}


func TestServiceRunShutdownPreservesPrimaryLifecycleAndDedicatedPostgresCleanupErrors(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	if transactionDB == nil {
		t.Fatal("expected dedicated transaction PostgreSQL database handle")
	}
	_, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if auditDB == nil {
		_ = transactionDB.Close()
		t.Fatal("expected dedicated audit PostgreSQL database handle")
	}
	if transactionDB == auditDB {
		_ = transactionDB.Close()
		_ = auditDB.Close()
		t.Fatal("dedicated transaction and audit stores must use independent database handles")
	}

	cleanupOrder := []string{}
	transactionCleanupErr := errors.New("injected dedicated transaction shutdown cleanup failure")
	auditCleanupErr := errors.New("injected dedicated audit shutdown cleanup failure")
	wrappedTransactionDB := &runtimeCleanupErrorDB{
		delegate: transactionDB,
		err: transactionCleanupErr,
		name: "transaction",
		order: &cleanupOrder,
	}
	wrappedAuditDB := &runtimeCleanupErrorDB{
		delegate: auditDB,
		err: auditCleanupErr,
		name: "audit",
		order: &cleanupOrder,
	}
	ownership := newRuntimeDatabaseOwnership(wrappedTransactionDB, wrappedAuditDB)
	ownership.transferToService()

	registry := provider.NewRegistry()
	balanceProvider := &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}
	if err := registry.Register("mock", balanceProvider); err != nil {
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
	service := &Service{
		syncService: syncService,
		balanceLifecycle: balanceLifecycle,
		databaseOwnership: ownership,
		interval: time.Hour,
	}

	runCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		result <- service.Run(runCtx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !balanceLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for balance worker to start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected primary lifecycle cancellation error, got %v", err)
		}
		if !errors.Is(err, transactionCleanupErr) {
			t.Fatalf("expected transaction cleanup error to remain discoverable, got %v", err)
		}
		if !errors.Is(err, auditCleanupErr) {
			t.Fatalf("expected audit cleanup error to remain discoverable, got %v", err)
		}
		if !strings.Contains(err.Error(), "close transaction database") {
			t.Fatalf("expected transaction database cleanup context, got %v", err)
		}
		if !strings.Contains(err.Error(), "close audit database") {
			t.Fatalf("expected audit database cleanup context, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for service shutdown")
	}

	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected both dedicated databases to close exactly once: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected deterministic transaction-before-audit cleanup order, got %v", cleanupOrder)
	}
	if !ownership.transferred() {
		t.Fatal("expected database ownership to remain transferred during service shutdown")
	}
	if err := service.Close(); !errors.Is(err, transactionCleanupErr) || !errors.Is(err, auditCleanupErr) {
		t.Fatalf("repeated service close must preserve both stored cleanup errors without closing again, got %v", err)
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected repeated service close not to close dedicated databases again: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL database handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL database handle to be closed")
	}
}


func TestServiceClosePreservesDedicatedPostgresCleanupErrorsAfterExplicitWorkerShutdown(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if transactionDB == nil || auditDB == nil || transactionDB == auditDB {
		if transactionDB != nil {
			_ = transactionDB.Close()
		}
		if auditDB != nil {
			_ = auditDB.Close()
		}
		t.Fatal("expected independent dedicated transaction and audit PostgreSQL handles")
	}

	transactionCleanupErr := errors.New("injected explicit transaction shutdown cleanup failure")
	auditCleanupErr := errors.New("injected explicit audit shutdown cleanup failure")
	cleanupOrder := []string{}
	wrappedTransactionDB := &runtimeCleanupErrorDB{
		delegate: transactionDB,
		err:      transactionCleanupErr,
		name:     "transaction",
		order:    &cleanupOrder,
	}
	wrappedAuditDB := &runtimeCleanupErrorDB{
		delegate: auditDB,
		err:      auditCleanupErr,
		name:     "audit",
		order:    &cleanupOrder,
	}
	ownership := newRuntimeDatabaseOwnership(wrappedTransactionDB, wrappedAuditDB)
	ownership.transferToService()

	registry := provider.NewRegistry()
	balanceProvider := &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}
	if err := registry.Register("mock", balanceProvider); err != nil {
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
	service := &Service{
		syncService:       syncService,
		balanceLifecycle:  balanceLifecycle,
		databaseOwnership: ownership,
		interval:          time.Hour,
	}

	if err := balanceLifecycle.Start(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !balanceLifecycle.Running() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for balance worker to start")
		}
		time.Sleep(time.Millisecond)
	}

	workerShutdownCtx, workerShutdownCancel := context.WithTimeout(ctx, time.Second)
	defer workerShutdownCancel()
	if err := balanceLifecycle.Shutdown(workerShutdownCtx); err != nil {
		t.Fatalf("expected explicit worker shutdown to succeed before service close, got %v", err)
	}
	if balanceLifecycle.Running() {
		t.Fatal("expected worker to stop before explicit service close")
	}

	shutdownErr := service.Close()
	if !errors.Is(shutdownErr, transactionCleanupErr) {
		t.Fatalf("expected explicit shutdown to preserve transaction cleanup error, got %v", shutdownErr)
	}
	if !errors.Is(shutdownErr, auditCleanupErr) {
		t.Fatalf("expected explicit shutdown to preserve audit cleanup error, got %v", shutdownErr)
	}
	if !strings.Contains(shutdownErr.Error(), "close transaction database") {
		t.Fatalf("expected transaction cleanup context, got %v", shutdownErr)
	}
	if !strings.Contains(shutdownErr.Error(), "close audit database") {
		t.Fatalf("expected audit cleanup context, got %v", shutdownErr)
	}
	if balanceLifecycle.Running() {
		t.Fatal("expected explicit service close to stop the owned worker")
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected dedicated databases to close exactly once: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected transaction-before-audit cleanup order, got %v", cleanupOrder)
	}

	repeatedErr := service.Close()
	if !errors.Is(repeatedErr, transactionCleanupErr) || !errors.Is(repeatedErr, auditCleanupErr) {
		t.Fatalf("repeated explicit close must preserve stored cleanup errors, got %v", repeatedErr)
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("repeated explicit close must not close databases again: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL handle to be closed")
	}
}


func TestServiceRunCatalogStartFailurePreservesPrimaryRollbackAndDedicatedPostgresCleanupErrors(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if transactionDB == nil || auditDB == nil || transactionDB == auditDB {
		if transactionDB != nil {
			_ = transactionDB.Close()
		}
		if auditDB != nil {
			_ = auditDB.Close()
		}
		t.Fatal("expected independent dedicated transaction and audit PostgreSQL handles")
	}

	transactionCleanupErr := errors.New("injected catalog rollback transaction cleanup failure")
	auditCleanupErr := errors.New("injected catalog rollback audit cleanup failure")
	cleanupOrder := []string{}
	wrappedTransactionDB := &runtimeCleanupErrorDB{
		delegate: transactionDB,
		err:      transactionCleanupErr,
		name:     "transaction",
		order:    &cleanupOrder,
	}
	wrappedAuditDB := &runtimeCleanupErrorDB{
		delegate: auditDB,
		err:      auditCleanupErr,
		name:     "audit",
		order:    &cleanupOrder,
	}
	ownership := newRuntimeDatabaseOwnership(wrappedTransactionDB, wrappedAuditDB)
	ownership.transferToService()

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

	primaryErr := errors.New("injected catalog lifecycle start failure")
	service := &Service{
		syncService:       syncService,
		catalogSync:       catalogSync,
		databaseOwnership: ownership,
		balanceLifecycle:  balanceLifecycle,
		interval:          time.Hour,
		catalogInterval:   time.Hour,
		catalogStart: func(context.Context) (context.Context, error) {
			return nil, primaryErr
		},
	}

	result := make(chan error, 1)
	go func() {
		result <- service.Run(ctx)
	}()

	select {
	case runErr := <-result:
		if !errors.Is(runErr, primaryErr) {
			t.Fatalf("expected primary catalog lifecycle error, got %v", runErr)
		}
		if !errors.Is(runErr, transactionCleanupErr) {
			t.Fatalf("expected transaction cleanup error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, auditCleanupErr) {
			t.Fatalf("expected audit cleanup error to remain discoverable, got %v", runErr)
		}
		if !strings.Contains(runErr.Error(), "close transaction database") {
			t.Fatalf("expected transaction cleanup context, got %v", runErr)
		}
		if !strings.Contains(runErr.Error(), "close audit database") {
			t.Fatalf("expected audit cleanup context, got %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for catalog rollback shutdown")
	}

	if balanceLifecycle.Running() {
		t.Fatal("expected balance worker to be rolled back after catalog start failure")
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected dedicated databases to close exactly once: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected transaction-before-audit cleanup order, got %v", cleanupOrder)
	}
	if !ownership.transferred() {
		t.Fatal("expected database ownership to remain transferred during runtime rollback")
	}

	repeatedErr := service.Close()
	if !errors.Is(repeatedErr, transactionCleanupErr) || !errors.Is(repeatedErr, auditCleanupErr) {
		t.Fatalf("repeated service close must preserve both cleanup errors, got %v", repeatedErr)
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("repeated service close must not close databases again: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL database handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL database handle to be closed")
	}
}


func TestServiceRunCatalogStartFailurePreservesWorkerRollbackAndPostgresCleanupErrors(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if transactionDB == nil || auditDB == nil || transactionDB == auditDB {
		if transactionDB != nil {
			_ = transactionDB.Close()
		}
		if auditDB != nil {
			_ = auditDB.Close()
		}
		t.Fatal("expected independent dedicated transaction and audit PostgreSQL handles")
	}

	transactionCleanupErr := errors.New("injected worker rollback transaction cleanup failure")
	auditCleanupErr := errors.New("injected worker rollback audit cleanup failure")
	workerRollbackErr := errors.New("injected balance worker rollback failure")
	primaryErr := errors.New("injected catalog lifecycle start failure")
	cleanupOrder := []string{}
	wrappedTransactionDB := &runtimeCleanupErrorDB{
		delegate: transactionDB,
		err:      transactionCleanupErr,
		name:     "transaction",
		order:    &cleanupOrder,
	}
	wrappedAuditDB := &runtimeCleanupErrorDB{
		delegate: auditDB,
		err:      auditCleanupErr,
		name:     "audit",
		order:    &cleanupOrder,
	}
	ownership := newRuntimeDatabaseOwnership(wrappedTransactionDB, wrappedAuditDB)
	ownership.transferToService()

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

	service := &Service{
		syncService:       syncService,
		catalogSync:       catalogSync,
		databaseOwnership: ownership,
		balanceLifecycle:  balanceLifecycle,
		interval:          time.Hour,
		catalogInterval:   time.Hour,
		catalogStart: func(context.Context) (context.Context, error) {
			return nil, primaryErr
		},
		balanceShutdown: func(ctx context.Context) error {
			shutdownErr := balanceLifecycle.Shutdown(ctx)
			if shutdownErr == nil {
				return workerRollbackErr
			}
			return errors.Join(shutdownErr, workerRollbackErr)
		},
	}

	result := make(chan error, 1)
	go func() {
		result <- service.Run(ctx)
	}()

	select {
	case runErr := <-result:
		if !errors.Is(runErr, primaryErr) {
			t.Fatalf("expected primary catalog lifecycle error, got %v", runErr)
		}
		if !errors.Is(runErr, workerRollbackErr) {
			t.Fatalf("expected worker rollback error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, transactionCleanupErr) {
			t.Fatalf("expected transaction cleanup error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, auditCleanupErr) {
			t.Fatalf("expected audit cleanup error to remain discoverable, got %v", runErr)
		}
		if !strings.Contains(runErr.Error(), "close transaction database") {
			t.Fatalf("expected transaction cleanup context, got %v", runErr)
		}
		if !strings.Contains(runErr.Error(), "close audit database") {
			t.Fatalf("expected audit cleanup context, got %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for catalog rollback shutdown")
	}

	if balanceLifecycle.Running() {
		t.Fatal("expected balance worker to be stopped before database cleanup")
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected dedicated databases to close exactly once: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected transaction-before-audit cleanup order, got %v", cleanupOrder)
	}
	if !ownership.transferred() {
		t.Fatal("expected database ownership to remain transferred during runtime rollback")
	}

	repeatedErr := service.Close()
	if !errors.Is(repeatedErr, transactionCleanupErr) || !errors.Is(repeatedErr, auditCleanupErr) {
		t.Fatalf("repeated service close must preserve both cleanup errors, got %v", repeatedErr)
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("repeated service close must not close databases again: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL handle to be closed")
	}
}


func TestServiceRunCancellationPreservesWorkerRollbackAndDedicatedPostgresCleanupErrors(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if transactionDB == nil || auditDB == nil || transactionDB == auditDB {
		if transactionDB != nil {
			_ = transactionDB.Close()
		}
		if auditDB != nil {
			_ = auditDB.Close()
		}
		t.Fatal("expected independent dedicated transaction and audit PostgreSQL handles")
	}

	transactionCleanupErr := errors.New("injected cancellation transaction cleanup failure")
	auditCleanupErr := errors.New("injected cancellation audit cleanup failure")
	workerRollbackErr := errors.New("injected cancellation balance worker rollback failure")
	cleanupOrder := []string{}
	wrappedTransactionDB := &runtimeCleanupErrorDB{
		delegate: transactionDB,
		err:      transactionCleanupErr,
		name:     "transaction",
		order:    &cleanupOrder,
	}
	wrappedAuditDB := &runtimeCleanupErrorDB{
		delegate: auditDB,
		err:      auditCleanupErr,
		name:     "audit",
		order:    &cleanupOrder,
	}
	ownership := newRuntimeDatabaseOwnership(wrappedTransactionDB, wrappedAuditDB)
	ownership.transferToService()

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

	service := &Service{
		syncService:       syncService,
		catalogSync:       catalogSync,
		databaseOwnership: ownership,
		balanceLifecycle:  balanceLifecycle,
		catalogLifecycle:  newCatalogWorkerLifecycle(),
		interval:          time.Hour,
		catalogInterval:   time.Hour,
		balanceShutdown: func(ctx context.Context) error {
			shutdownErr := balanceLifecycle.Shutdown(ctx)
			if shutdownErr == nil {
				return workerRollbackErr
			}
			return errors.Join(shutdownErr, workerRollbackErr)
		},
	}

	runCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		result <- service.Run(runCtx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for balance and catalog workers to start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected primary cancellation error, got %v", runErr)
		}
		if !errors.Is(runErr, workerRollbackErr) {
			t.Fatalf("expected worker rollback error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, transactionCleanupErr) {
			t.Fatalf("expected transaction cleanup error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, auditCleanupErr) {
			t.Fatalf("expected audit cleanup error to remain discoverable, got %v", runErr)
		}
		if !strings.Contains(runErr.Error(), "close transaction database") {
			t.Fatalf("expected transaction cleanup context, got %v", runErr)
		}
		if !strings.Contains(runErr.Error(), "close audit database") {
			t.Fatalf("expected audit cleanup context, got %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cancellation shutdown")
	}

	if balanceLifecycle.Running() {
		t.Fatal("expected balance worker to stop during cancellation rollback")
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to stop during cancellation rollback")
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected dedicated databases to close exactly once: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected transaction-before-audit cleanup order, got %v", cleanupOrder)
	}
	if !ownership.transferred() {
		t.Fatal("expected database ownership to remain transferred during cancellation shutdown")
	}

	repeatedErr := service.Close()
	if !errors.Is(repeatedErr, transactionCleanupErr) || !errors.Is(repeatedErr, auditCleanupErr) {
		t.Fatalf("repeated service close must preserve both cleanup errors, got %v", repeatedErr)
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("repeated service close must not close databases again: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL handle to be closed")
	}
}

func TestServiceRunBalanceStartFailurePreservesDedicatedPostgresCleanupErrors(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if transactionDB == nil || auditDB == nil || transactionDB == auditDB {
		if transactionDB != nil {
			_ = transactionDB.Close()
		}
		if auditDB != nil {
			_ = auditDB.Close()
		}
		t.Fatal("expected independent dedicated transaction and audit PostgreSQL handles")
	}

	primaryErr := errors.New("injected balance worker start failure")
	transactionCleanupErr := errors.New("injected balance start transaction cleanup failure")
	auditCleanupErr := errors.New("injected balance start audit cleanup failure")
	cleanupOrder := []string{}
	wrappedTransactionDB := &runtimeCleanupErrorDB{
		delegate: transactionDB,
		err:      transactionCleanupErr,
		name:     "transaction",
		order:    &cleanupOrder,
	}
	wrappedAuditDB := &runtimeCleanupErrorDB{
		delegate: auditDB,
		err:      auditCleanupErr,
		name:     "audit",
		order:    &cleanupOrder,
	}
	ownership := newRuntimeDatabaseOwnership(wrappedTransactionDB, wrappedAuditDB)
	ownership.transferToService()

	registry := provider.NewRegistry()
	balanceProvider := &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}
	if err := registry.Register("mock", balanceProvider); err != nil {
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

	service := &Service{
		syncService:       syncService,
		databaseOwnership: ownership,
		balanceLifecycle:  balanceLifecycle,
		interval:          time.Hour,
		balanceStart: func(context.Context) error {
			return primaryErr
		},
	}

	runErr := service.Run(ctx)
	if !errors.Is(runErr, primaryErr) {
		t.Fatalf("expected primary balance worker start error, got %v", runErr)
	}
	if !errors.Is(runErr, transactionCleanupErr) {
		t.Fatalf("expected transaction cleanup error to remain discoverable, got %v", runErr)
	}
	if !errors.Is(runErr, auditCleanupErr) {
		t.Fatalf("expected audit cleanup error to remain discoverable, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "close transaction database") {
		t.Fatalf("expected transaction cleanup context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "close audit database") {
		t.Fatalf("expected audit cleanup context, got %v", runErr)
	}
	if balanceLifecycle.Running() {
		t.Fatal("expected balance worker to remain stopped after injected start failure")
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected dedicated databases to close exactly once: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected transaction-before-audit cleanup order, got %v", cleanupOrder)
	}
	if !ownership.transferred() {
		t.Fatal("expected transferred database ownership to remain transferred during startup rollback")
	}

	repeatedErr := service.Close()
	if !errors.Is(repeatedErr, transactionCleanupErr) || !errors.Is(repeatedErr, auditCleanupErr) {
		t.Fatalf("repeated service close must preserve both cleanup errors, got %v", repeatedErr)
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("repeated service close must not close databases again: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL database handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL database handle to be closed")
	}
}

func TestServiceRunCancellationPreservesCatalogShutdownCompletionAndDedicatedPostgresCleanupErrors(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	transactionCfg := Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn}
	auditCfg := Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}
	_, transactionDB, err := openTransactionStore(ctx, transactionCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, auditDB, err := openAuditStore(ctx, auditCfg, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if transactionDB == nil || auditDB == nil || transactionDB == auditDB {
		if transactionDB != nil {
			_ = transactionDB.Close()
		}
		if auditDB != nil {
			_ = auditDB.Close()
		}
		t.Fatal("expected independent dedicated transaction and audit PostgreSQL handles")
	}

	catalogShutdownErr := errors.New("injected catalog shutdown completion failure")
	transactionCleanupErr := errors.New("injected catalog shutdown transaction cleanup failure")
	auditCleanupErr := errors.New("injected catalog shutdown audit cleanup failure")
	workerRollbackErr := errors.New("injected catalog shutdown balance worker rollback failure")
	cleanupOrder := []string{}
	wrappedTransactionDB := &runtimeCleanupErrorDB{
		delegate: transactionDB,
		err:      transactionCleanupErr,
		name:     "transaction",
		order:    &cleanupOrder,
	}
	wrappedAuditDB := &runtimeCleanupErrorDB{
		delegate: auditDB,
		err:      auditCleanupErr,
		name:     "audit",
		order:    &cleanupOrder,
	}
	ownership := newRuntimeDatabaseOwnership(wrappedTransactionDB, wrappedAuditDB)
	ownership.transferToService()

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

	service := &Service{
		syncService:       syncService,
		catalogSync:       catalogSync,
		databaseOwnership: ownership,
		balanceLifecycle:  balanceLifecycle,
		catalogLifecycle:  newCatalogWorkerLifecycle(),
		interval:          time.Hour,
		catalogInterval:   time.Hour,
		balanceShutdown: func(ctx context.Context) error {
			shutdownErr := balanceLifecycle.Shutdown(ctx)
			if shutdownErr == nil {
				return workerRollbackErr
			}
			return errors.Join(shutdownErr, workerRollbackErr)
		},
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		return catalogShutdownErr
	}

	runCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		result <- service.Run(runCtx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for balance and catalog workers to start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected primary cancellation error, got %v", runErr)
		}
		if !errors.Is(runErr, workerRollbackErr) {
			t.Fatalf("expected worker rollback error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, catalogShutdownErr) {
			t.Fatalf("expected catalog shutdown completion error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, transactionCleanupErr) {
			t.Fatalf("expected transaction cleanup error to remain discoverable, got %v", runErr)
		}
		if !errors.Is(runErr, auditCleanupErr) {
			t.Fatalf("expected audit cleanup error to remain discoverable, got %v", runErr)
		}
		if !strings.Contains(runErr.Error(), "close transaction database") {
			t.Fatalf("expected transaction cleanup context, got %v", runErr)
		}
		message := runErr.Error()
		orderedNeedles := []string{
			"context canceled",
			"injected catalog shutdown balance worker rollback failure",
			"injected catalog shutdown completion failure",
			"close transaction database: injected catalog shutdown transaction cleanup failure",
			"close audit database: injected catalog shutdown audit cleanup failure",
		}
		last := -1
		for _, needle := range orderedNeedles {
			index := strings.Index(message, needle)
			if index < 0 {
				t.Fatalf("expected composed error to contain %q: %v", needle, runErr)
			}
			if index <= last {
				t.Fatalf("expected PostgreSQL shutdown error precedence order to remain stable, got %q", message)
			}
			last = index
		}
		if !strings.Contains(runErr.Error(), "close audit database") {
			t.Fatalf("expected audit cleanup context, got %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cancellation shutdown")
	}

	if balanceLifecycle.Running() {
		t.Fatal("expected balance worker to stop during cancellation rollback")
	}
	if service.catalogLifecycle.Running() {
		t.Fatal("expected catalog lifecycle to stop during cancellation rollback")
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("expected dedicated databases to close exactly once: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if len(cleanupOrder) != 2 || cleanupOrder[0] != "transaction" || cleanupOrder[1] != "audit" {
		t.Fatalf("expected transaction-before-audit cleanup order, got %v", cleanupOrder)
	}
	if !ownership.transferred() {
		t.Fatal("expected database ownership to remain transferred during cancellation shutdown")
	}

	repeatedErr := service.Close()
	if !errors.Is(repeatedErr, transactionCleanupErr) || !errors.Is(repeatedErr, auditCleanupErr) {
		t.Fatalf("repeated service close must preserve both cleanup errors, got %v", repeatedErr)
	}
	if errors.Is(repeatedErr, catalogShutdownErr) {
		t.Fatal("repeated Service.Close must not replay catalog shutdown completion error")
	}
	if wrappedTransactionDB.closeCount != 1 || wrappedAuditDB.closeCount != 1 {
		t.Fatalf("repeated service close must not close databases again: tx=%d audit=%d", wrappedTransactionDB.closeCount, wrappedAuditDB.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL database handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL database handle to be closed")
	}
}


func TestServiceRunSuccessfulShutdownOrderingAndDedicatedPostgresSingleClose(t *testing.T) {
	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	ctx := context.Background()
	_, transactionDB, err := openTransactionStore(ctx, Config{TransactionStoreDriver: "postgres", PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	_, auditDB, err := openAuditStore(ctx, Config{AuditStoreDriver: "postgres", PostgresDSN: dsn}, nil)
	if err != nil {
		_ = transactionDB.Close()
		t.Fatal(err)
	}
	if transactionDB == nil || auditDB == nil || transactionDB == auditDB {
		if transactionDB != nil {
			_ = transactionDB.Close()
		}
		if auditDB != nil {
			_ = auditDB.Close()
		}
		t.Fatal("expected independent dedicated transaction and audit PostgreSQL handles")
	}

	order := []string{}
	transactionWrapped := &runtimeCleanupErrorDB{delegate: transactionDB, name: "transaction", order: &order}
	auditWrapped := &runtimeCleanupErrorDB{delegate: auditDB, name: "audit", order: &order}
	ownership := newRuntimeDatabaseOwnership(transactionWrapped, auditWrapped)
	ownership.transferToService()

	registry := provider.NewRegistry()
	if err := registry.Register("mock", &balanceMock{Provider: mock.New(mock.Config{}), balance: 100000}); err != nil {
		t.Fatal(err)
	}
	syncService, err := operational.NewSyncService(registry, operational.NewMemoryStore(), "IDR", 1)
	if err != nil {
		t.Fatal(err)
	}
	catalogSync, err := catalog.NewSyncService(registry, catalog.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	balanceLifecycle, err := operational.NewSyncWorkerLifecycle(syncService, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	service := &Service{
		syncService:       syncService,
		catalogSync:       catalogSync,
		databaseOwnership: ownership,
		balanceLifecycle:  balanceLifecycle,
		catalogLifecycle:  newCatalogWorkerLifecycle(),
		interval:          time.Hour,
		catalogInterval:   time.Hour,
		balanceShutdown: func(ctx context.Context) error {
			err := balanceLifecycle.Shutdown(ctx)
			order = append(order, "balance")
			return err
		},
	}
	service.catalogShutdown = func() error {
		service.catalogLifecycle.Shutdown()
		order = append(order, "catalog")
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() {
		result <- service.Run(runCtx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !balanceLifecycle.Running() || !service.catalogLifecycle.Running() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for balance and catalog workers to start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}

	wantOrder := []string{"balance", "catalog", "transaction", "audit"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("unexpected successful shutdown ordering: got %v want %v", order, wantOrder)
	}
	if transactionWrapped.closeCount != 1 || auditWrapped.closeCount != 1 {
		t.Fatalf("expected one close per dedicated PostgreSQL handle, got tx=%d audit=%d", transactionWrapped.closeCount, auditWrapped.closeCount)
	}
	if service.balanceLifecycle.Running() || service.catalogLifecycle.Running() {
		t.Fatal("expected all service lifecycles to be stopped")
	}

	if err := service.Close(); err != nil {
		t.Fatalf("expected repeated Close to remain successful, got %v", err)
	}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("repeated Close must not replay lifecycle or database completion, got %v", order)
	}
	if transactionWrapped.closeCount != 1 || auditWrapped.closeCount != 1 {
		t.Fatalf("repeated Close must not double-close dedicated PostgreSQL handles, got tx=%d audit=%d", transactionWrapped.closeCount, auditWrapped.closeCount)
	}
	if err := transactionDB.PingContext(ctx); err == nil {
		t.Fatal("expected transaction PostgreSQL handle to be closed")
	}
	if err := auditDB.PingContext(ctx); err == nil {
		t.Fatal("expected audit PostgreSQL handle to be closed")
	}
}
