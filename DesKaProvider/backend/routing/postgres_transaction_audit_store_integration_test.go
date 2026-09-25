package routing

import (
	"context"
	"errors"
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
	events, err := restarted.AllContext(ctx, "audit-ref")
	if err != nil {
		t.Fatalf("read audit events after restart: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two durable audit events, got %d", len(events))
	}
	if events[0] != first || events[1] != second {
		t.Fatalf("durable audit events changed: %#v", events)
	}

	other, err := restarted.AllContext(ctx, "other-ref")
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
