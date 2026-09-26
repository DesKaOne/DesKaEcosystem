package routing

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	Mock "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/Mock"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
)

func TestPostgresTransactionAuditStoreAppendOnlyDurabilityAndRestartRead(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_store_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	created1 := time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond)
	created2 := created1.Add(time.Second)
	first := TransactionAuditEvent{
		ReferenceID: "audit-ref",
		Action: "PURCHASE_PENDING",
		Next: string(provider.StatusPending),
		ProviderName: "mock",
		Message: "pending",
		CreatedAt: created1,
	}
	second := TransactionAuditEvent{
		ReferenceID: "audit-ref",
		Action: "PURCHASE_RESULT",
		Previous: string(provider.StatusPending),
		Next: string(provider.StatusSuccess),
		ProviderName: "mock",
		Message: "success",
		CreatedAt: created2,
	}
	if err := store.AppendContext(ctx, first); err != nil {
		t.Fatalf("append first audit event: %v", err)
	}
	if err := store.AppendContext(ctx, second); err != nil {
		t.Fatalf("append second audit event: %v", err)
	}

	restarted, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	events, err := restarted.AllContextE(ctx, "audit-ref")
	if err != nil {
		t.Fatalf("read audit events after restart: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two durable audit events, got %d", len(events))
	}
	if events[0] != first || events[1] != second {
		t.Fatalf("durable audit events changed: %#v", events)
	}

	other, err := restarted.AllContextE(ctx, "other-ref")
	if err != nil {
		t.Fatalf("read unrelated audit events: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("expected no unrelated audit events, got %d", len(other))
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_transaction_audit WHERE reference_id=$1", "audit-ref").Scan(&count); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected exactly two append-only rows, got %d", count)
	}
}

func TestPostgresTransactionAuditStoreFailureDoesNotChangeTransactionState(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_failure_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	transactionStore, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	auditStore, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}

	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{
		Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode: "00",
		Message: "success",
		PurchaseStatus: provider.StatusSuccess,
		Price: 20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance: 100000,
		Health: operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, operationalStore, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewServiceWithStoreContextAndAudit(ctx, router, transactionStore, auditStore)
	if err != nil {
		t.Fatal(err)
	}
	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo: "08123456789",
		ReferenceID: "audit-failure-ref",
		Amount: 20000,
	}
	result, err := service.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("purchase with healthy audit store: %v", err)
	}
	if result.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected terminal success, got %q", result.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected one provider submission, got %d", got)
	}

	// A canceled audit write must fail before touching the audit table. The
	// already durable transaction state remains authoritative.
	canceled, cancelAudit := context.WithCancel(context.Background())
	cancelAudit()
	err = auditStore.AppendContext(canceled, TransactionAuditEvent{
		ReferenceID: req.ReferenceID,
		Action: "AUDIT_FAILURE_TEST",
		CreatedAt: time.Now().UTC(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled audit append, got %v", err)
	}
	durable, ok := transactionStore.Get(req.ReferenceID)
	if !ok || durable.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("audit failure must not change durable terminal state: %#v", durable)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("audit failure must not authorize resubmission, got %d", got)
	}
}


func TestPostgresTransactionAndAuditStoresReconstructServiceAfterRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "transaction_audit_restart_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	transactionStore, err := NewPostgresTransactionStore(db)
	if err != nil {
		t.Fatal(err)
	}
	auditStore, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}

	registry := provider.NewRegistry()
	mock := Mock.New(Mock.Config{
		Products: []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode: "00",
		Message: "success",
		PurchaseStatus: provider.StatusSuccess,
		Price: 20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	operationalStore := operational.NewMemoryStore()
	if err := operationalStore.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance: 100000,
		Health: operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, operationalStore, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo: "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount: 20000,
	}
	firstService, err := NewServiceWithStoreContextAndAudit(ctx, router, transactionStore, auditStore)
	if err != nil {
		t.Fatalf("construct initial service: %v", err)
	}
	first, err := firstService.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("initial purchase: %v", err)
	}
	if first.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected terminal success, got %q", first.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected exactly one provider submission, got %d", got)
	}

	beforeRestart, err := auditStore.AllContextE(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history before restart: %v", err)
	}
	if len(beforeRestart) != 2 || beforeRestart[0].Action != "PURCHASE_PENDING" || beforeRestart[1].Action != "PURCHASE_RESULT" {
		t.Fatalf("unexpected pre-restart audit history: %#v", beforeRestart)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("reopen postgres: %v", err)
	}
	defer reopened.Close()
	if err := reopened.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened postgres: %v", err)
	}
	if _, err := reopened.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("restore search path after restart: %v", err)
	}

	restartedTransactionStore, err := NewPostgresTransactionStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	restartedAuditStore, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := NewServiceWithStoreContextAndAudit(ctx, router, restartedTransactionStore, restartedAuditStore)
	if err != nil {
		t.Fatalf("reconstruct service after restart: %v", err)
	}

	reconciled, err := restarted.Reconcile(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("reconcile recovered terminal transaction: %v", err)
	}
	if !samePurchaseResult(reconciled.Result, first.Result) {
		t.Fatalf("terminal result changed after restart: %#v", reconciled.Result)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("restart reconciliation must not resubmit purchase, got %d", got)
	}

	afterRestart, err := restartedAuditStore.AllContextE(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("read durable audit history after restart: %v", err)
	}
	if len(afterRestart) != len(beforeRestart) {
		t.Fatalf("idempotent terminal reconciliation must not append a new audit event, before=%d after=%d", len(beforeRestart), len(afterRestart))
	}
	for i := range beforeRestart {
		if afterRestart[i] != beforeRestart[i] {
			t.Fatalf("audit history changed after restart: before=%#v after=%#v", beforeRestart, afterRestart)
		}
	}

	webhook := provider.WebhookEvent{
		ReferenceID: req.ReferenceID,
		ProductCode: req.ProductCode,
		CustomerNo: req.CustomerNo,
		Status: provider.StatusSuccess,
		ProviderCode: first.Result.ProviderCode,
		Message: first.Result.Message,
		SerialNumber: first.Result.SerialNumber,
		Price: first.Result.Price,
	}
	if _, err := restarted.HandleWebhook(ctx, webhook); err != nil {
		t.Fatalf("identical terminal webhook after restart must be idempotent: %v", err)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("terminal webhook after restart must not resubmit purchase, got %d", got)
	}
}



func TestPostgresTransactionAuditStoreAppendFailureThenRecovery(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_append_recovery_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	event := TransactionAuditEvent{
		ReferenceID:  "audit-append-recovery-ref",
		Action:       "PURCHASE_RESULT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "recovered append",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	err = store.AppendContext(context.Background(), event)
	if err == nil {
		t.Fatal("expected audit append failure after database close")
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("closed-database append failure must not be classified as context termination: %v", err)
	}

	reopened, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("reopen postgres: %v", err)
	}
	defer reopened.Close()
	if err := reopened.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened postgres: %v", err)
	}
	if _, err := reopened.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("restore search path after recovery: %v", err)
	}

	recoveredStore, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	if err := recoveredStore.AppendContext(ctx, event); err != nil {
		t.Fatalf("append audit event after database recovery: %v", err)
	}

	events, err := recoveredStore.AllContextE(ctx, event.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history after recovered append: %v", err)
	}
	if len(events) != 1 || events[0] != event {
		t.Fatalf("recovered append must produce exactly one durable audit event, got %#v", events)
	}

	var count int
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_transaction_audit WHERE reference_id=$1", event.ReferenceID).Scan(&count); err != nil {
		t.Fatalf("count recovered audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("recovered append must not duplicate the audit event, got %d rows", count)
	}
}


func TestPostgresTransactionAuditStoreReadFailureThenRecovery(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_recovery_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	store, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	event := TransactionAuditEvent{
		ReferenceID:  "audit-recovery-ref",
		Action:       "PURCHASE_RESULT",
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "recovered",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := store.AppendContext(ctx, event); err != nil {
		t.Fatalf("append recovery fixture: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	failed, err := store.AllContextE(context.Background(), event.ReferenceID)
	if err == nil {
		t.Fatal("expected audit read failure after database close")
	}
	if failed != nil {
		t.Fatalf("failed audit read must not expose partial history, got %#v", failed)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("closed-database failure must not be classified as context termination: %v", err)
	}

	reopened, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("reopen postgres: %v", err)
	}
	defer reopened.Close()
	if err := reopened.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened postgres: %v", err)
	}
	if _, err := reopened.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("restore search path after recovery: %v", err)
	}

	recoveredStore, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := recoveredStore.AllContextE(ctx, event.ReferenceID)
	if err != nil {
		t.Fatalf("audit read after database recovery: %v", err)
	}
	if len(recovered) != 1 || recovered[0] != event {
		t.Fatalf("recovered audit history changed: %#v", recovered)
	}
}
