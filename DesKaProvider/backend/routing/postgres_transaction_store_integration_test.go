package routing

import (
	"context"
	"database/sql"
	"errors"
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

func applyPostgresMigration(t *testing.T, db DBTX) {
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

func seedTerminalTransactionContext(ctx context.Context, store *PostgresTransactionStore, state TransactionState) error {
	pending := state
	pending.Version = 1
	pending.Execution.Result.Status = provider.StatusPending
	pending.Execution.Result.ProviderCode = "00"
	pending.Execution.Result.Message = "pending"
	pending.Execution.Result.SerialNumber = ""
	if err := store.PutContext(ctx, pending); err != nil {
		return err
	}
	if state.Execution.Result.Status == provider.StatusPending {
		return nil
	}
	return store.PutIfCurrentContext(ctx, state.Request.ReferenceID, pending, state)
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

	// search_path is session-local. Pin a dedicated database session for the
	// concurrent atomic transition so both workers address the same schema.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin postgres connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET search_path TO public"); err != nil {
		t.Fatalf("set pinned search path: %v", err)
	}
	store, err = NewPostgresTransactionStore(conn)
	if err != nil {
		t.Fatal(err)
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
			results <- store.PutIfCurrent(current.Request.ReferenceID, current, success)
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

	// Keep concurrent transition workers on one connection pool scoped to this schema.
	pinnedDB, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("open pinned postgres connection: %v", err)
	}
	t.Cleanup(func() { _ = pinnedDB.Close() })
	if err := pinnedDB.PingContext(ctx); err != nil {
		t.Fatalf("ping pinned postgres: %v", err)
	}
	if _, err := pinnedDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set pinned search path: %v", err)
	}
	pinnedStore, err := NewPostgresTransactionStore(pinnedDB)
	if err != nil {
		t.Fatal(err)
	}

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
		Health:        operational.HealthHealthy,
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

func TestPostgresTransactionStoreFailedRecoveryIsIdempotentAfterRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "failed_recovery_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
		ProviderCode:   "02",
		Message:        "failed",
		PurchaseStatus: provider.StatusFailed,
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
		t.Fatalf("initial failed purchase: %v", err)
	}
	if first.Result.Status != provider.StatusFailed {
		t.Fatalf("expected initial purchase to be failed, got %q", first.Result.Status)
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
		t.Fatalf("reconcile recovered failed transaction: %v", err)
	}
	if !samePurchaseResult(reconciled.Result, first.Result) {
		t.Fatalf("failed terminal result changed after restart reconciliation: %#v", reconciled.Result)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("failed terminal reconciliation must not resubmit purchase, got %d submissions", got)
	}

	event := provider.WebhookEvent{
		ReferenceID:  req.ReferenceID,
		ProductCode:  req.ProductCode,
		CustomerNo:   req.CustomerNo,
		Status:       provider.StatusFailed,
		ProviderCode: first.Result.ProviderCode,
		Message:      first.Result.Message,
		SerialNumber: first.Result.SerialNumber,
		Price:        first.Result.Price,
	}
	webhookResult, err := restarted.HandleWebhook(ctx, event)
	if err != nil {
		t.Fatalf("identical terminal webhook must be idempotent: %v", err)
	}
	if !samePurchaseResult(webhookResult.Result, first.Result) {
		t.Fatalf("terminal webhook changed durable result: %#v", webhookResult.Result)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("terminal webhook must not resubmit purchase, got %d submissions", got)
	}

	durable, ok := store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("failed terminal transaction disappeared after restart")
	}
	if !samePurchaseResult(durable.Execution.Result, first.Result) {
		t.Fatalf("durable failed result changed: %#v", durable.Execution.Result)
	}
}


func TestPostgresTransactionStoreTerminalConflictsAfterRestartDoNotResubmit(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "terminal_conflict_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	if ok := mock.SetTransactionStatus(req.ReferenceID, provider.StatusFailed, "provider reports failed"); !ok {
		t.Fatal("failed to mutate mock provider observation for conflict scenario")
	}

	_, err = restarted.Reconcile(ctx, req.ReferenceID)
	if !errors.Is(err, ErrWebhookReferenceConflict) {
		t.Fatalf("expected reconciliation conflict after terminal observation changed, got %v", err)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("reconciliation conflict must not resubmit purchase, got %d submissions", got)
	}

	conflictingWebhook := provider.WebhookEvent{
		ReferenceID:  req.ReferenceID,
		ProductCode:  req.ProductCode,
		CustomerNo:   req.CustomerNo,
		Status:       provider.StatusFailed,
		ProviderCode: "02",
		Message:      "provider reports failed",
		Price:        first.Result.Price,
	}
	_, err = restarted.HandleWebhook(ctx, conflictingWebhook)
	if !errors.Is(err, ErrWebhookReferenceConflict) {
		t.Fatalf("expected conflicting terminal webhook to be rejected, got %v", err)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("conflicting terminal webhook must not resubmit purchase, got %d submissions", got)
	}

	durable, ok := store.Get(req.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction disappeared after conflict handling")
	}
	if !samePurchaseResult(durable.Execution.Result, first.Result) {
		t.Fatalf("terminal durable result changed after conflicting observations: %#v", durable.Execution.Result)
	}
}

func TestPostgresConcurrentPutInitialCreationConverges(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "initial_create_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	state := TransactionState{
		Request: PurchaseRequest{
			ProductCode: "pln20",
			CustomerNo: "08123456789",
			ReferenceID: postgresIntegrationReference(),
			Amount: 20000,
		},
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ProductCode: "pln20",
				CustomerNo: "08123456789",
				Status: provider.StatusPending,
			},
		},
		Version: 1,
	}
	state.Execution.Result.ReferenceID = state.Request.ReferenceID

	const workers = 2
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.PutContext(ctx, state)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent idempotent initial creation failed: %v", err)
		}
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_transactions WHERE reference_id = $1", state.Request.ReferenceID).Scan(&count); err != nil {
		t.Fatalf("count persisted initial transaction: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one durable transaction row, got %d", count)
	}

	stored, ok := store.Get(state.Request.ReferenceID)
	if !ok {
		t.Fatal("initial transaction was not persisted")
	}
	if stored.Request != state.Request || stored.Execution.ProviderName != state.Execution.ProviderName || stored.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("persisted initial transaction identity/state changed: %#v", stored)
	}
}

func TestPostgresConcurrentServiceReconcileConvergesWithoutResubmission(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "service_reconcile_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set search path: %v", err)
	}
	applyPostgresMigration(t, db)

	openStore := func() *PostgresTransactionStore {
		conn, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
		if err != nil {
			t.Fatalf("open concurrent postgres connection: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		conn.SetMaxOpenConns(1)
		conn.SetMaxIdleConns(1)
		if err := conn.PingContext(ctx); err != nil {
			t.Fatalf("ping concurrent postgres connection: %v", err)
		}
		if _, err := conn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
			t.Fatalf("set concurrent search path: %v", err)
		}
		store, err := NewPostgresTransactionStore(conn)
		if err != nil {
			t.Fatal(err)
		}
		return store
	}

	initialStore := openStore()
	firstStore := openStore()
	secondStore := openStore()

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
	initial, err := NewServiceWithStoreContext(ctx, router, initialStore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := initial.Purchase(ctx, req); err != nil {
		t.Fatalf("initial pending purchase: %v", err)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected exactly one provider submission before concurrent reconciliation, got %d", got)
	}

	mock.SetTransactionStatus(req.ReferenceID, provider.StatusSuccess, "success")

	first, err := NewServiceWithStoreContext(ctx, router, firstStore)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewServiceWithStoreContext(ctx, router, secondStore)
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		execution PurchaseExecution
		err       error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, service := range []*Service{first, second} {
		wg.Add(1)
		go func(service *Service) {
			defer wg.Done()
			execution, err := service.Reconcile(ctx, req.ReferenceID)
			results <- result{execution: execution, err: err}
		}(service)
	}
	wg.Wait()
	close(results)

	var firstResult PurchaseExecution
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err != nil {
			durable, durableOK := firstStore.Get(req.ReferenceID)
			t.Fatalf("concurrent service reconciliation failed: %v; durable=%#v durableOK=%t", got.err, durable, durableOK)
		}
		if i == 0 {
			firstResult = got.execution
			continue
		}
		if !samePurchaseResult(got.execution.Result, firstResult.Result) {
			t.Fatalf("concurrent reconciliation results diverged: %#v != %#v", got.execution.Result, firstResult.Result)
		}
	}
	if firstResult.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected concurrent reconciliation to converge to success, got %#v", firstResult)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("concurrent reconciliation must not resubmit provider purchase, got %d submissions", got)
	}

	durable, ok := firstStore.Get(req.ReferenceID)
	if !ok {
		t.Fatal("durable transaction disappeared after concurrent reconciliation")
	}
	if durable.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected durable success after concurrent reconciliation, got %q", durable.Execution.Result.Status)
	}
}
func TestPostgresTransactionStoreSequentialVersionTransitionIsPreserved(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "sequential_version_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
	}
	pending := TransactionState{
		Request: req,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				Status:       provider.StatusPending,
				ProviderCode: "00",
				Message:      "pending",
				Price:        20000,
			},
		},
		Version: 1,
	}
	if err := store.PutContext(ctx, pending); err != nil {
		t.Fatalf("insert pending transaction: %v", err)
	}

	pendingRefresh := pending
	pendingRefresh.Execution.Result.Message = "still pending"
	if err := store.PutContext(ctx, pendingRefresh); err != nil {
		t.Fatalf("sequential pending transition: %v", err)
	}
	current, ok := store.GetContext(ctx, req.ReferenceID)
	if !ok {
		t.Fatal("pending transaction disappeared after sequential transition")
	}
	if current.Version != 2 {
		t.Fatalf("expected version 2 after pending transition, got %d", current.Version)
	}
	if current.Execution.Result.Message != "still pending" {
		t.Fatalf("pending transition did not persist message: %#v", current.Execution.Result)
	}

	success := current
	success.Execution.Result.Status = provider.StatusSuccess
	success.Execution.Result.Message = "success"
	if err := store.PutContext(ctx, success); err != nil {
		t.Fatalf("sequential terminal transition: %v", err)
	}
	terminal, ok := store.GetContext(ctx, req.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction disappeared after sequential transition")
	}
	if terminal.Version != 3 {
		t.Fatalf("expected version 3 after terminal transition, got %d", terminal.Version)
	}
	if terminal.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected success terminal state, got %q", terminal.Execution.Result.Status)
	}

	stale := pendingRefresh
	stale.Execution.Result.Message = "stale update"
	if err := store.PutIfCurrentContext(ctx, req.ReferenceID, stale, success); err != ErrTransactionStateConflict {
		t.Fatalf("expected stale version conflict after terminal transition, got %v", err)
	}
}

func TestPostgresTransactionStoreContextReadHonorsDeadline(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.GetContextE(ctx, "deadline-read"); err == nil {
		t.Fatal("expected deadline GetContextE to return an error")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded from GetContextE, got %v", err)
	}
	if _, err := store.AllContextE(ctx); err == nil {
		t.Fatal("expected deadline AllContextE to return an error")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded from AllContextE, got %v", err)
	}
}

func TestPostgresTransactionStoreContextWriteHonorsCancellation(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	pending := postgresPendingState()
	pending.Request.ReferenceID = postgresIntegrationReference()
	pending.Execution.Result.ReferenceID = pending.Request.ReferenceID

	if err := store.PutContext(ctx, pending); err == nil {
		t.Fatal("expected canceled PutContext to return an error")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from PutContext, got %v", err)
	}

	if _, ok := store.GetContext(ctx, pending.Request.ReferenceID); ok {
		t.Fatal("canceled PutContext must not persist the transaction")
	}
}


func TestPostgresTransactionStorePutContextAdvancesPendingVersion(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "pending_version_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	pending := postgresPendingState()
	pending.Request.ReferenceID = postgresIntegrationReference()
	pending.Execution.Result.ReferenceID = pending.Request.ReferenceID
	if err := store.PutContext(ctx, pending); err != nil {
		t.Fatalf("insert initial pending transaction: %v", err)
	}

	first := pending
	first.Execution.Result.Message = "pending refresh 1"
	if err := store.PutContext(ctx, first); err != nil {
		t.Fatalf("advance pending state once: %v", err)
	}

	current, ok := store.GetContext(ctx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction missing after first transition")
	}
	if current.Version != 2 {
		t.Fatalf("expected version 2 after first pending transition, got %d", current.Version)
	}

	second := current
	second.Execution.Result.Message = "pending refresh 2"
	if err := store.PutContext(ctx, second); err != nil {
		t.Fatalf("advance pending state twice: %v", err)
	}

	recovered, ok := store.GetContext(ctx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction missing after second transition")
	}
	if recovered.Version != 3 {
		t.Fatalf("expected version 3 after second pending transition, got %d", recovered.Version)
	}
	if recovered.Execution.Result.Message != "pending refresh 2" {
		t.Fatalf("expected latest pending message to persist, got %q", recovered.Execution.Result.Message)
	}
}

func TestPostgresTransactionStoreContextAtomicWriteHonorsCancellation(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	pending := postgresPendingState()
	pending.Request.ReferenceID = postgresIntegrationReference()
	pending.Execution.Result.ReferenceID = pending.Request.ReferenceID
	if err := store.PutContext(ctx, pending); err != nil {
		t.Fatalf("insert pending transaction: %v", err)
	}

	current, ok := store.GetContext(ctx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction was not persisted")
	}
	success := current
	success.Execution.Result.Status = provider.StatusSuccess
	success.Execution.Result.ProviderCode = "00"
	success.Execution.Result.Message = "success"

	cancel()

	if err := store.PutIfCurrentContext(ctx, pending.Request.ReferenceID, current, success); err == nil {
		t.Fatal("expected canceled PutIfCurrentContext to return an error")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from PutIfCurrentContext, got %v", err)
	}

	checkCtx, checkCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer checkCancel()
	recovered, ok := store.GetContext(checkCtx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("transaction disappeared after canceled atomic write")
	}
	if recovered.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("canceled atomic write must leave pending state unchanged, got %q", recovered.Execution.Result.Status)
	}
}

func TestPostgresTransactionStoreContextAtomicWriteCancellationPreservesVersion(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	pending := postgresPendingState()
	pending.Request.ReferenceID = postgresIntegrationReference()
	pending.Execution.Result.ReferenceID = pending.Request.ReferenceID
	if err := store.PutContext(ctx, pending); err != nil {
		t.Fatalf("insert pending transaction: %v", err)
	}

	current, ok := store.GetContext(ctx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction was not persisted")
	}
	if current.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", current.Version)
	}

	next := current
	next.Execution.Result.Message = "cancelled transition"
	cancelledCtx, cancelled := context.WithCancel(context.Background())
	cancelled()

	if err := store.PutContext(cancelledCtx, next); err == nil {
		t.Fatal("expected canceled PutContext to return an error")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from PutContext, got %v", err)
	}

	recovered, ok := store.GetContext(ctx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("transaction disappeared after canceled PutContext")
	}
	if recovered.Version != 1 {
		t.Fatalf("canceled PutContext must not advance version, got %d", recovered.Version)
	}
	if recovered.Execution.Result.Message != current.Execution.Result.Message {
		t.Fatalf("canceled PutContext changed durable message: %q", recovered.Execution.Result.Message)
	}
}

func TestPostgresTransactionStoreContextReadHonorsCancellation(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.GetContextE(ctx, "cancelled-read"); err == nil {
		t.Fatal("expected canceled GetContextE to return an error")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from GetContextE, got %v", err)
	}
	if _, err := store.AllContextE(ctx); err == nil {
		t.Fatal("expected canceled AllContextE to return an error")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled from AllContextE, got %v", err)
	}
}


type postgresExecErrorDB struct {
	err error
}

func (d postgresExecErrorDB) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, d.err
}

func (postgresExecErrorDB) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("unexpected QueryContext call")
}

func (postgresExecErrorDB) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return nil
}

func TestPostgresTransactionStoreAtomicWritePreservesDatabaseErrorClassification(t *testing.T) {
	dbErr := errors.New("simulated database outage")
	store, err := NewPostgresTransactionStore(postgresExecErrorDB{err: dbErr})
	if err != nil {
		t.Fatal(err)
	}
	pending := postgresPendingState()
	pending.Request.ReferenceID = postgresIntegrationReference()
	pending.Execution.Result.ReferenceID = pending.Request.ReferenceID
	next := pending
	next.Execution.Result.Status = provider.StatusSuccess
	next.Execution.Result.ProviderCode = "00"
	next.Execution.Result.Message = "success"

	err = store.PutIfCurrentContext(context.Background(), pending.Request.ReferenceID, pending, next)
	if err == nil {
		t.Fatal("expected database error from atomic transition")
	}
	if errors.Is(err, ErrTransactionStateConflict) {
		t.Fatalf("database error must not be classified as transaction-state conflict: %v", err)
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("database error was not preserved through wrapping: %v", err)
	}
}

func TestPostgresTransactionStoreReadConsistencyPreservesDurableIdentity(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "read_consistency_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
		Testing:     true,
	}
	terminal := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID:  request.ReferenceID,
				ProductCode:  request.ProductCode,
				CustomerNo:   request.CustomerNo,
				Status:       provider.StatusSuccess,
				ProviderCode: "00",
				Message:      "success",
				SerialNumber: "SN-READ-1",
				Price:        20000,
			},
		},
		Version: 1,
	}
	if err := store.PutContext(ctx, terminal); err != nil {
		t.Fatalf("persist terminal transaction: %v", err)
	}

	got, ok, err := store.GetContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read terminal transaction: %v", err)
	}
	if !ok {
		t.Fatal("expected terminal transaction to be found")
	}
	if got.Request != terminal.Request {
		t.Fatalf("durable request identity changed on read: got=%#v want=%#v", got.Request, terminal.Request)
	}
	if got.Execution.ProviderName != terminal.Execution.ProviderName {
		t.Fatalf("durable provider identity changed on read: got=%q want=%q", got.Execution.ProviderName, terminal.Execution.ProviderName)
	}
	if !samePurchaseResult(got.Execution.Result, terminal.Execution.Result) {
		t.Fatalf("durable result identity changed on read: got=%#v want=%#v", got.Execution.Result, terminal.Execution.Result)
	}
	if got.Version != terminal.Version {
		t.Fatalf("durable version changed on read: got=%d want=%d", got.Version, terminal.Version)
	}
}

func TestPostgresConcurrentReadDuringAtomicTransitionObservesCompleteState(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "read_transition_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	seedStore, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
	}
	pending := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				ProductCode: request.ProductCode,
				CustomerNo: request.CustomerNo,
				Status: provider.StatusPending,
				ProviderCode: "00",
				Message: "pending",
				Price: 20000,
			},
		},
		Version: 1,
	}
	if err := seedStore.PutContext(ctx, pending); err != nil {
		t.Fatalf("seed pending transaction: %v", err)
	}

	readerDB, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("open reader postgres: %v", err)
	}
	t.Cleanup(func() { _ = readerDB.Close() })
	readerDB.SetMaxOpenConns(1)
	readerDB.SetMaxIdleConns(1)
	if err := readerDB.PingContext(ctx); err != nil {
		t.Fatalf("ping reader postgres: %v", err)
	}
	if _, err := readerDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set reader search path: %v", err)
	}
	readerStore, err := NewPostgresTransactionStore(readerDB)
	if err != nil {
		t.Fatal(err)
	}

	const reads = 100
	readerReady := make(chan struct{})
	startReads := make(chan struct{})
	readDone := make(chan error, 1)
	go func() {
		close(readerReady)
		<-startReads
		for i := 0; i < reads; i++ {
			got, ok, err := readerStore.GetContextE(ctx, request.ReferenceID)
			if err != nil {
				readDone <- fmt.Errorf("read iteration %d: %w", i, err)
				return
			}
			if !ok {
				readDone <- fmt.Errorf("read iteration %d: transaction missing", i)
				return
			}
			if got.Request != pending.Request || got.Execution.ProviderName != pending.Execution.ProviderName {
				readDone <- fmt.Errorf("read iteration %d observed mixed identity fields: %#v", i, got)
				return
			}
			status := got.Execution.Result.Status
			if status != provider.StatusPending && status != provider.StatusSuccess {
				readDone <- fmt.Errorf("read iteration %d observed invalid intermediate status %q", i, status)
				return
			}
			if status == provider.StatusPending {
				if got.Version != 1 || got.Execution.Result.Message != "pending" || got.Execution.Result.ProviderCode != "00" || got.Execution.Result.Price != 20000 {
					readDone <- fmt.Errorf("read iteration %d observed inconsistent pending state: %#v", i, got)
					return
				}
			} else {
				if got.Version != 2 || got.Execution.Result.Message != "success" || got.Execution.Result.ProviderCode != "00" || got.Execution.Result.SerialNumber != "SN-ATOMIC-1" || got.Execution.Result.Price != 20000 {
					readDone <- fmt.Errorf("read iteration %d observed inconsistent terminal state: %#v", i, got)
					return
				}
			}
		}
		readDone <- nil
	}()

	<-readerReady
	close(startReads)

	next := pending
	next.Version = 2
	next.Execution.Result.Status = provider.StatusSuccess
	next.Execution.Result.Message = "success"
	next.Execution.Result.SerialNumber = "SN-ATOMIC-1"
	if err := seedStore.PutIfCurrentContext(ctx, request.ReferenceID, pending, next); err != nil {
		t.Fatalf("atomic terminal transition: %v", err)
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTerminalReadRemainsIdempotentAcrossRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "terminal_read_restart_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
	}
	terminal := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				ProductCode: request.ProductCode,
				CustomerNo: request.CustomerNo,
				Status: provider.StatusSuccess,
				ProviderCode: "00",
				Message: "success",
				SerialNumber: "SN-RESTART-1",
				Price: 20000,
			},
		},
		Version: 2,
	}
	if err := seedTerminalTransactionContext(ctx, store, terminal); err != nil {
		t.Fatalf("seed terminal transaction: %v", err)
	}

	read := func(s *PostgresTransactionStore) TransactionState {
		got, ok, err := s.GetContextE(ctx, request.ReferenceID)
		if err != nil {
			t.Fatalf("read terminal transaction: %v", err)
		}
		if !ok {
			t.Fatal("terminal transaction missing")
		}
		return got
	}

	first := read(store)
	if !samePurchaseResult(first.Execution.Result, terminal.Execution.Result) || first.Version != terminal.Version ||
		first.Request != terminal.Request || first.Execution.ProviderName != terminal.Execution.ProviderName {
		t.Fatalf("initial terminal read changed durable state: got=%#v want=%#v", first, terminal)
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
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set restarted search path: %v", err)
	}
	restarted, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		got := read(restarted)
		if !samePurchaseResult(got.Execution.Result, terminal.Execution.Result) || got.Version != terminal.Version ||
			got.Request != terminal.Request || got.Execution.ProviderName != terminal.Execution.ProviderName {
			t.Fatalf("restart read %d changed committed terminal state: got=%#v want=%#v", i, got, terminal)
		}
	}

	durable, ok, err := restarted.GetContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("final durable read: %v", err)
	}
	if !ok {
		t.Fatal("final terminal transaction missing")
	}
	if durable.Version != terminal.Version || !samePurchaseResult(durable.Execution.Result, terminal.Execution.Result) {
		t.Fatalf("repeated terminal reads changed persisted version/result: got=%#v want=%#v", durable, terminal)
	}
}
 
func TestPostgresTerminalResultRemainsImmutableAgainstStaleObservation(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "terminal_immutable_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
	}
	pending := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				ProductCode: request.ProductCode,
				CustomerNo: request.CustomerNo,
				Status: provider.StatusPending,
				ProviderCode: "00",
				Message: "pending",
				Price: 20000,
			},
		},
		Version: 1,
	}
	if err := store.PutContext(ctx, pending); err != nil {
		t.Fatalf("seed pending transaction: %v", err)
	}

	committed := pending
	committed.Version = 2
	committed.Execution.Result.Status = provider.StatusSuccess
	committed.Execution.Result.Message = "success"
	committed.Execution.Result.SerialNumber = "SN-IMMUTABLE-1"
	if err := store.PutIfCurrentContext(ctx, request.ReferenceID, pending, committed); err != nil {
		t.Fatalf("commit terminal result: %v", err)
	}

	stale := pending
	stale.Version = 2
	stale.Execution.Result.Status = provider.StatusFailed
	stale.Execution.Result.Message = "stale"
	stale.Execution.Result.SerialNumber = "SN-STALE"
	if err := store.PutIfCurrentContext(ctx, request.ReferenceID, pending, stale); err != ErrTransactionStateConflict {
		t.Fatalf("expected stale terminal transition to conflict, got %v", err)
	}

	got, ok, err := store.GetContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read committed terminal transaction: %v", err)
	}
	if !ok {
		t.Fatal("committed terminal transaction disappeared")
	}
	if !samePurchaseResult(got.Execution.Result, committed.Execution.Result) || got.Version != committed.Version {
		t.Fatalf("terminal result/version mutated after stale observation: got=%#v want=%#v", got, committed)
	}
}

func TestPostgresRestartReadConcurrentObservation(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "restart_read_concurrent_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	openStore := func() *PostgresTransactionStore {
		conn, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
		if err != nil {
			t.Fatalf("open restart-read postgres: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		conn.SetMaxOpenConns(1)
		conn.SetMaxIdleConns(1)
		if err := conn.PingContext(ctx); err != nil {
			t.Fatalf("ping restart-read postgres: %v", err)
		}
		if _, err := conn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
			t.Fatalf("set restart-read search path: %v", err)
		}
		store, err := NewPostgresTransactionStore(conn)
		if err != nil {
			t.Fatal(err)
		}
		return store
	}

	initialStore := openStore()
	state := postgresPendingState()
	state.Request.ReferenceID = postgresIntegrationReference()
	state.Execution.Result.ReferenceID = state.Request.ReferenceID
	if err := initialStore.PutContext(ctx, state); err != nil {
		t.Fatalf("persist pending transaction: %v", err)
	}

	durablePending, ok := initialStore.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction missing before terminal commit")
	}
	terminal := durablePending
	terminal.Execution.Result.Status = provider.StatusSuccess
	terminal.Execution.Result.ProviderCode = "00"
	terminal.Execution.Result.Message = "success"
	terminal.Execution.Result.Price = 20000
	if err := initialStore.PutIfCurrentContext(ctx, state.Request.ReferenceID, durablePending, terminal); err != nil {
		t.Fatalf("commit terminal transaction: %v", err)
	}

	_ = db.Close()
	db, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("reopen postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened postgres: %v", err)
	}

	firstStore := openStore()
	secondStore := openStore()

	before, ok := firstStore.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction missing after restart")
	}

	const workers = 2
	results := make(chan TransactionState, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for _, store := range []*PostgresTransactionStore{firstStore, secondStore} {
		wg.Add(1)
		go func(store *PostgresTransactionStore) {
			defer wg.Done()
			got, ok := store.GetContext(ctx, state.Request.ReferenceID)
			if !ok {
				errs <- fmt.Errorf("transaction missing after concurrent restart read")
				return
			}
			results <- got
			errs <- nil
		}(store)
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent restart read failed: %v", err)
		}
	}

	count := 0
	for got := range results {
		if got.Request != before.Request ||
			got.Execution.ProviderName != before.Execution.ProviderName ||
			!samePurchaseResult(got.Execution.Result, before.Execution.Result) ||
			got.Version != before.Version {
			t.Fatalf("concurrent restart read diverged from committed terminal state: before=%#v got=%#v", before, got)
		}
		count++
	}
	if count != workers {
		t.Fatalf("expected %d concurrent reads, got %d", workers, count)
	}

	stale := before
	stale.Execution.Result.Status = provider.StatusFailed
	stale.Execution.Result.ProviderCode = "99"
	stale.Execution.Result.Message = "stale"
	if err := firstStore.PutIfCurrentContext(ctx, state.Request.ReferenceID, before, stale); !errors.Is(err, ErrReferenceConflict) {
		t.Fatalf("stale observation must remain non-authoritative after restart, got %v", err)
	}

	after, ok := secondStore.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction disappeared after stale observation")
	}
	if after.Request != before.Request ||
		after.Execution.ProviderName != before.Execution.ProviderName ||
		!samePurchaseResult(after.Execution.Result, before.Execution.Result) ||
		after.Version != before.Version {
		t.Fatalf("committed terminal state changed after stale observation: before=%#v after=%#v", before, after)
	}
}

func TestPostgresContextCancellationPreservesDurableState(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "ctx_cancel_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	conn := db
	store, err := NewPostgresTransactionStore(conn)
	if err != nil {
		t.Fatal(err)
	}

	state := postgresPendingState()
	state.Request.ReferenceID = postgresIntegrationReference()
	state.Execution.Result.ReferenceID = state.Request.ReferenceID
	if err := store.PutContext(ctx, state); err != nil {
		t.Fatalf("persist pending transaction: %v", err)
	}

	durable, ok := store.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction missing before cancellation checks")
	}
	terminal := durable
	terminal.Execution.Result.Status = provider.StatusSuccess
	terminal.Execution.Result.ProviderCode = "00"
	terminal.Execution.Result.Message = "success"
	terminal.Execution.Result.Price = 20000
	if err := store.PutIfCurrentContext(ctx, state.Request.ReferenceID, durable, terminal); err != nil {
		t.Fatalf("commit terminal transaction: %v", err)
	}

	cancelledRead, cancelRead := context.WithCancel(context.Background())
	cancelRead()
	if _, ok := store.GetContext(cancelledRead, state.Request.ReferenceID); ok {
		t.Fatal("canceled read must not return an authoritative transaction state")
	}

	before, ok := store.GetContext(context.Background(), state.Request.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction missing after canceled read")
	}

	canceledWrite, cancelWrite := context.WithCancel(context.Background())
	cancelWrite()
	stale := before
	stale.Execution.Result.Status = provider.StatusFailed
	stale.Execution.Result.ProviderCode = "99"
	stale.Execution.Result.Message = "canceled"
	if err := store.PutIfCurrentContext(canceledWrite, state.Request.ReferenceID, before, stale); err == nil {
		t.Fatal("canceled transition unexpectedly succeeded")
	}

	after, ok := store.GetContext(context.Background(), state.Request.ReferenceID)
	if !ok {
		t.Fatal("terminal transaction disappeared after canceled transition")
	}
	if after.Request != before.Request ||
		after.Execution.ProviderName != before.Execution.ProviderName ||
		!samePurchaseResult(after.Execution.Result, before.Execution.Result) ||
		after.Version != before.Version {
		t.Fatalf("canceled operations changed durable state: before=%#v after=%#v", before, after)
	}
}

func TestPostgresSequentialPendingTransitionVersionIntegrity(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "sequential_version_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	state := postgresPendingState()
	state.Request.ReferenceID = postgresIntegrationReference()
	state.Execution.Result.ReferenceID = state.Request.ReferenceID
	if err := store.PutContext(ctx, state); err != nil {
		t.Fatalf("persist initial pending transaction: %v", err)
	}

	first, ok := store.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("initial pending transaction missing")
	}
	if first.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", first.Version)
	}

	second := first
	second.Execution.Result.Message = "pending-refresh-1"
	if err := store.PutContext(ctx, second); err != nil {
		t.Fatalf("persist first pending transition: %v", err)
	}

	afterFirst, ok := store.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("transaction missing after first pending transition")
	}
	if afterFirst.Version != first.Version+1 {
		t.Fatalf("expected version to advance after first pending transition, before=%d after=%d", first.Version, afterFirst.Version)
	}
	if afterFirst.Execution.Result.Status != provider.StatusPending || afterFirst.Execution.Result.Message != "pending-refresh-1" {
		t.Fatalf("unexpected first pending transition state: %#v", afterFirst)
	}

	third := afterFirst
	third.Execution.Result.Message = "pending-refresh-2"
	if err := store.PutContext(ctx, third); err != nil {
		t.Fatalf("persist second pending transition: %v", err)
	}

	afterSecond, ok := store.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("transaction missing after second pending transition")
	}
	if afterSecond.Version != afterFirst.Version+1 {
		t.Fatalf("expected version to advance sequentially, first=%d second=%d", afterFirst.Version, afterSecond.Version)
	}
	if afterSecond.Execution.Result.Message != "pending-refresh-2" {
		t.Fatalf("unexpected second pending transition state: %#v", afterSecond)
	}

	stale := afterFirst
	stale.Execution.Result.Message = "stale-write"
	if err := store.PutIfCurrentContext(ctx, state.Request.ReferenceID, afterFirst, stale); !errors.Is(err, ErrTransactionStateConflict) {
		t.Fatalf("expected stale version transition to conflict, got %v", err)
	}

	final, ok := store.GetContext(ctx, state.Request.ReferenceID)
	if !ok {
		t.Fatal("transaction missing after stale version rejection")
	}
	if final.Version != afterSecond.Version || final.Execution.Result.Message != afterSecond.Execution.Result.Message {
		t.Fatalf("stale version changed durable state: before=%#v after=%#v", afterSecond, final)
	}
}
func TestPostgresConcurrentReadTransitionObservesCompleteState(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "read_transition_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})

	migrationConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin migration connection: %v", err)
	}
	defer migrationConn.Close()
	if _, err := migrationConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set isolated search path: %v", err)
	}
	applyPostgresMigration(t, migrationConn)

	openStore := func(role string) (*sql.Conn, *PostgresTransactionStore) {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("pin %s connection: %v", role, err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		if _, err := conn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
			t.Fatalf("set %s search path: %v", role, err)
		}
		store, err := NewPostgresTransactionStore(conn)
		if err != nil {
			t.Fatalf("new %s store: %v", role, err)
		}
		return conn, store
	}
	_, reader := openStore("reader")
	_, writer := openStore("writer")

	pending := postgresPendingState()
	pending.Request.ReferenceID = postgresIntegrationReference()
	pending.Execution.ProviderName = "mock"
	pending.Execution.Result.ReferenceID = pending.Request.ReferenceID
	pending.Execution.Result.ProviderCode = "00"
	pending.Execution.Result.Message = "pending"
	pending.Execution.Result.SerialNumber = "SN-PENDING"
	pending.Execution.Result.Price = 20000
	pending.Version = 1
	if err := writer.PutContext(ctx, pending); err != nil {
		t.Fatalf("insert pending: %v", err)
	}

	success := pending
	success.Execution.Result.Status = provider.StatusSuccess
	success.Execution.Result.Message = "success"
	success.Execution.Result.SerialNumber = "SN-SUCCESS"

	const readers = 24
	reads := make(chan TransactionState, readers)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, store := openStore(fmt.Sprintf("reader-%d", i))
			state, ok := store.GetContext(ctx, pending.Request.ReferenceID)
			if ok {
				reads <- state
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := writer.PutIfCurrentContext(ctx, pending.Request.ReferenceID, pending, success); err != nil {
			errs <- err
		}
	}()
	wg.Wait()
	close(reads)
	close(errs)

	for err := range errs {
		t.Fatalf("atomic transition failed: %v", err)
	}
	for state := range reads {
		if state.Request != pending.Request || state.Execution.ProviderName != pending.Execution.ProviderName {
			t.Fatalf("read observed mixed identity fields: %#v", state)
		}
		result := state.Execution.Result
		validPending := result.Status == provider.StatusPending && result.Message == "pending" && result.SerialNumber == "SN-PENDING" && state.Version == 1
		validSuccess := result.Status == provider.StatusSuccess && result.Message == "success" && result.SerialNumber == "SN-SUCCESS" && state.Version == 2
		if !validPending && !validSuccess {
			t.Fatalf("read observed partial transition state: %#v", state)
		}
	}

	final, ok := reader.GetContext(ctx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("final transaction disappeared")
	}
	if final.Version != 2 || final.Execution.Result.Status != provider.StatusSuccess || final.Execution.Result.Message != "success" || final.Execution.Result.SerialNumber != "SN-SUCCESS" {
		t.Fatalf("unexpected final transaction state: %#v", final)
	}
}


func TestPostgresPutContextAdvancesVersionAcrossRepeatedTransitions(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	applyPostgresMigration(t, db)
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
	pending.Version = 1
	if err := store.PutContext(ctx, pending); err != nil {
		t.Fatalf("insert pending transaction: %v", err)
	}

	pendingRefresh := pending
	pendingRefresh.Execution.Result.Message = "still pending"
	if err := store.PutContext(ctx, pendingRefresh); err != nil {
		t.Fatalf("refresh pending transaction: %v", err)
	}
	current, ok, err := store.GetContextE(ctx, pending.Request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read refreshed transaction: ok=%v err=%v", ok, err)
	}
	if current.Version != 2 || current.Execution.Result.Message != "still pending" {
		t.Fatalf("expected version 2 pending refresh, got %#v", current)
	}

	success := current
	success.Execution.Result.Status = provider.StatusSuccess
	success.Execution.Result.ProviderCode = "00"
	success.Execution.Result.Message = "success"
	success.Execution.Result.SerialNumber = "SN-V3"
	if err := store.PutContext(ctx, success); err != nil {
		t.Fatalf("advance transaction from version 2 to terminal state: %v", err)
	}

	final, ok, err := store.GetContextE(ctx, pending.Request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read final transaction: ok=%v err=%v", ok, err)
	}
	if final.Version != 3 || final.Execution.Result.Status != provider.StatusSuccess || final.Execution.Result.SerialNumber != "SN-V3" {
		t.Fatalf("expected version 3 success state, got %#v", final)
	}
}

func TestPostgresCanceledContextDoesNotWriteTransaction(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	applyPostgresMigration(t, db)
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

	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := store.PutContext(canceled, pending); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled PutContext, got %v", err)
	}
	if _, ok, err := store.GetContextE(ctx, pending.Request.ReferenceID); err != nil {
		t.Fatalf("read transaction after canceled PutContext: %v", err)
	} else if ok {
		t.Fatal("canceled PutContext must not create a durable transaction")
	}

	activeCtx, activeCancel := context.WithCancel(ctx)
	defer activeCancel()
	if err := store.PutContext(activeCtx, pending); err != nil {
		t.Fatalf("insert pending transaction: %v", err)
	}
	current, ok, err := store.GetContextE(ctx, pending.Request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read inserted transaction: ok=%v err=%v", ok, err)
	}
	success := current
	success.Execution.Result.Status = provider.StatusSuccess
	success.Execution.Result.ProviderCode = "00"
	success.Execution.Result.Message = "success"

	canceledTransition, stopTransition := context.WithCancel(ctx)
	stopTransition()
	if err := store.PutIfCurrentContext(canceledTransition, pending.Request.ReferenceID, current, success); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled PutIfCurrentContext, got %v", err)
	}
	final, ok, err := store.GetContextE(ctx, pending.Request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read transaction after canceled transition: ok=%v err=%v", ok, err)
	}
	if final.Execution.Result.Status != provider.StatusPending || final.Version != current.Version {
		t.Fatalf("canceled transition changed durable state: %#v", final)
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
		"CREATE TABLE IF NOT EXISTS provider_transaction_audit",
		"provider_transaction_audit_reference_created_idx",
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


func TestPostgresConcurrentReadDuringAtomicTransitionSeesCompleteState(t *testing.T) {
	db := postgresIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	applyPostgresMigration(t, db)
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
	pending.Execution.Result.Status = provider.StatusPending
	pending.Execution.Result.ProviderCode = "00"
	pending.Execution.Result.Message = "pending"
	pending.Execution.Result.SerialNumber = ""
	pending.Execution.Result.Price = pending.Request.Amount
	if err := store.Put(pending); err != nil {
		t.Fatalf("insert pending transaction: %v", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin transition connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET search_path TO public"); err != nil {
		t.Fatalf("set transition search path: %v", err)
	}
	transitionStore, err := NewPostgresTransactionStore(conn)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := transitionStore.GetContext(ctx, pending.Request.ReferenceID)
	if !ok {
		t.Fatal("pending transaction was not visible on dedicated transition connection")
	}
	next := current
	next.Execution.Result.Status = provider.StatusSuccess
	next.Execution.Result.ProviderCode = "00"
	next.Execution.Result.Message = "success"
	next.Execution.Result.SerialNumber = "SN-ATOMIC"

	transitionStarted := make(chan struct{})
	transitionDone := make(chan error, 1)
	go func() {
		close(transitionStarted)
		transitionDone <- transitionStore.PutIfCurrentContext(ctx, current.Request.ReferenceID, current, next)
	}()
	<-transitionStarted

	const reads = 32
	for i := 0; i < reads; i++ {
		observed, ok, err := store.GetContextE(ctx, pending.Request.ReferenceID)
		if err != nil {
			t.Fatalf("read %d failed: %v", i, err)
		}
		if !ok {
			t.Fatalf("read %d lost transaction row", i)
		}
		result := observed.Execution.Result
		pendingComplete := result.Status == provider.StatusPending && result.ProviderCode == "00" && result.Message == "pending" && result.SerialNumber == "" && result.Price == pending.Request.Amount
		successComplete := result.Status == provider.StatusSuccess && result.ProviderCode == "00" && result.Message == "success" && result.SerialNumber == "SN-ATOMIC" && result.Price == pending.Request.Amount
		if !pendingComplete && !successComplete {
			t.Fatalf("read %d observed mixed transaction state: %#v", i, result)
		}
	}

	if err := <-transitionDone; err != nil {
		t.Fatalf("atomic transition failed: %v", err)
	}
	final, ok, err := store.GetContextE(ctx, pending.Request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read final transaction: ok=%v err=%v", ok, err)
	}
	if final.Execution.Result.Status != provider.StatusSuccess || final.Execution.Result.SerialNumber != "SN-ATOMIC" {
		t.Fatalf("expected committed complete success state, got %#v", final.Execution.Result)
	}
}
