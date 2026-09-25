package routing

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	Mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func postgresIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("DESKAPROVIDER_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DESKAPROVIDER_POSTGRES_DSN is not configured")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	return db
}

func postgresMigrationSQL(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	path := filepath.Join(filepath.Dir(file), "..", "migrations", "001_provider_transactions.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read postgres migration: %v", err)
	}
	return string(content)
}

func applyPostgresMigration(t *testing.T, db *sql.DB) {
	t.Helper()

	sqlText := postgresMigrationSQL(t)
	var uncommented []string
	for _, line := range strings.Split(sqlText, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		uncommented = append(uncommented, line)
	}
	var statements []string
	for _, raw := range strings.Split(strings.Join(uncommented, "\n"), ";") {
		statement := strings.TrimSpace(raw)
		if statement != "" {
			statements = append(statements, statement)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("apply postgres migration statement %q: %v", statement, err)
		}
	}
}

func postgresIntegrationReference() string {
	return fmt.Sprintf("pg-it-%d", time.Now().UnixNano())
}

func TestPostgresTransactionStoreIntegration(t *testing.T) {
	db := postgresIntegrationDB(t)
	applyPostgresMigration(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "TRUNCATE provider_transactions"); err != nil {
		t.Fatalf("truncate provider transactions: %v", err)
	}

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}

	pending := postgresPendingState()
	pending.Request.ReferenceID = postgresIntegrationReference()
	pending.Execution.Result.ReferenceID = pending.Request.ReferenceID
	if err := store.Put(pending); err != nil {
		t.Fatalf("insert pending transaction: %v", err)
	}

	current, ok := store.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction was not persisted")
	}
	if current.Request != pending.Request || current.Execution.ProviderName != pending.Execution.ProviderName {
		t.Fatalf("persisted identity mismatch: %#v", current)
	}

	success := current
	success.Execution.Result.Status = provider.StatusSuccess
	success.Execution.Result.ProviderCode = "00"
	success.Execution.Result.Message = "ok"
	success.Execution.Result.Price = 20000

	const workers = 2
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- store.PutIfCurrent(pending.Request.ReferenceID, pending, success)
		}()
	}
	wg.Wait()
	close(results)

	var successCount, conflictCount int
	for err := range results {
		switch {
		case err == nil:
			successCount++
		case err == ErrTransactionStateConflict:
			conflictCount++
		default:
			t.Fatalf("unexpected concurrent transition error: %v", err)
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("expected one success and one conflict, got success=%d conflict=%d", successCount, conflictCount)
	}

	terminal, ok := store.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction disappeared after concurrent transition")
	}
	if terminal.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected success after concurrent transition, got %s", terminal.Execution.Result.Status)
	}

	_ = db.Close()
	db, err = sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("reopen postgres: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened postgres: %v", err)
	}

	restarted, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	recovered, ok := restarted.Get(pending.Request.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction was not recoverable after reconnect")
	}
	if recovered.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected recovered success, got %s", recovered.Execution.Result.Status)
	}

	if err := restarted.PutIfCurrent(pending.Request.ReferenceID, pending, success); err != ErrTransactionStateConflict {
		t.Fatalf("expected stale pending transition to conflict after restart, got %v", err)
	}
}

func TestPostgresTransactionStoreTerminalRecoveryIsIdempotentAfterRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "terminal_recovery_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set isolated search path: %v", err)
	}
	applyPostgresMigration(t, db)

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}

	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "success",
		PurchaseStatus: provider.StatusSuccess,
		Price:          20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}

	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance:      100000,
		Health:       operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, operationalStore, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
	}
	firstService, err := NewServiceWithStoreContext(ctx, router, store)
	if err != nil {
		t.Fatalf("construct initial service: %v", err)
	}
	first, err := firstService.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("initial successful purchase: %v", err)
	}
	if first.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected initial purchase to be success, got %q", first.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected exactly one provider submission, got %d", got)
	}

	restarted, err := NewServiceWithStoreContext(ctx, router, store)
	if err != nil {
		t.Fatalf("reconstruct service from PostgreSQL: %v", err)
	}
	reconciled, err := restarted.Reconcile(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("reconcile recovered terminal transaction: %v", err)
	}
	if !samePurchaseResult(reconciled.Result, first.Result) {
		t.Fatalf("terminal result changed after restart reconciliation: %#v", reconciled.Result)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("terminal reconciliation must not resubmit purchase, got %d submissions", got)
	}

	terminal, ok := store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction disappeared after restart reconciliation")
	}
	if !samePurchaseResult(terminal.Execution.Result, first.Result) {
		t.Fatalf("durable terminal result changed: %#v", terminal.Execution.Result)
	}

	if err := store.PutIfCurrent(req.ReferenceID, terminal, terminal); err != nil {
		t.Fatalf("identical terminal transition must be idempotent, got %v", err)
	}

	failed := terminal
	failed.Execution.Result.Status = provider.StatusFailed
	failed.Execution.Result.ProviderCode = "02"
	failed.Execution.Result.Message = "failed"
	if err := store.PutIfCurrent(req.ReferenceID, terminal, failed); err != ErrReferenceConflict {
		t.Fatalf("terminal state must reject mutation after restart, got %v", err)
	}
}

func TestPostgresMigrationVerification(t *testing.T) {
	sqlText := postgresMigrationSQL(t)
	required := []string{
		"CREATE TABLE IF NOT EXISTS provider_transactions",
		"reference_id TEXT PRIMARY KEY",
		"status TEXT NOT NULL CHECK (status IN ('pending', 'success', 'failed'))",
		"version BIGINT NOT NULL DEFAULT 1",
		"AND version = $7",
		"AND status = 'pending'",
		"RETURNING *",
	}
	for _, fragment := range required {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("migration missing required fragment %q", fragment)
		}
	}
}


func TestPostgresTransactionStoreRestartRecoveryReconcilesWithoutResubmission(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "restart_recovery_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set isolated search path: %v", err)
	}
	applyPostgresMigration(t, db)

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}

	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "pending",
		PurchaseStatus: provider.StatusPending,
		Price:          20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}

	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance:      100000,
		Health:       operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, operationalStore, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
	}
	firstService, err := NewServiceWithStoreContext(ctx, router, store)
	if err != nil {
		t.Fatalf("construct initial service: %v", err)
	}
	first, err := firstService.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("initial purchase: %v", err)
	}
	if first.Result.Status != provider.StatusPending {
		t.Fatalf("expected initial purchase to remain pending, got %q", first.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected exactly one provider submission before restart, got %d", got)
	}

	restarted, err := NewServiceWithStoreContext(ctx, router, store)
	if err != nil {
		t.Fatalf("reconstruct service from PostgreSQL: %v", err)
	}

	reconciled, err := restarted.Reconcile(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("reconcile recovered pending transaction: %v", err)
	}
	if reconciled.ProviderName != first.ProviderName {
		t.Fatalf("provider identity changed during recovery: %q", reconciled.ProviderName)
	}
	if reconciled.Result.Status != provider.StatusPending {
		t.Fatalf("expected provider status to remain pending, got %q", reconciled.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("restart reconciliation must not resubmit purchase, got %d submissions", got)
	}

	recovered, ok := store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("recovered transaction disappeared")
	}
	if recovered.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("expected durable pending state after reconciliation, got %q", recovered.Execution.Result.Status)
	}
}
