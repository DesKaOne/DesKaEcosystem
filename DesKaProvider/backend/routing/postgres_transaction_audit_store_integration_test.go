package routing

import (
	"fmt"
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
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



func TestPostgresTransactionAuditStoreRepeatedIdenticalAppendRemainsAppendOnly(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_append_idempotency_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	event := TransactionAuditEvent{
		ReferenceID:  "audit-identical-append-ref",
		Action:       "PURCHASE_RESULT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "same evidence",
		CreatedAt:    createdAt,
	}

	if err := store.AppendContext(ctx, event); err != nil {
		t.Fatalf("append first identical audit event: %v", err)
	}
	if err := store.AppendContext(ctx, event); err != nil {
		t.Fatalf("append second identical audit event: %v", err)
	}

	events, err := store.AllContextE(ctx, event.ReferenceID)
	if err != nil {
		t.Fatalf("read repeated audit evidence: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("repeated identical audit appends must remain append-only evidence, got %d rows", len(events))
	}
	if events[0] != event || events[1] != event {
		t.Fatalf("repeated identical audit evidence changed: %#v", events)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_transaction_audit WHERE reference_id=$1", event.ReferenceID).Scan(&count); err != nil {
		t.Fatalf("count repeated audit rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("repeated identical audit appends must create two evidence rows, got %d", count)
	}

	var transactionCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_transactions WHERE reference_id=$1", event.ReferenceID).Scan(&transactionCount); err != nil {
		t.Fatalf("count unrelated transaction rows: %v", err)
	}
	if transactionCount != 0 {
		t.Fatalf("audit append must not create transaction authority state, got %d rows", transactionCount)
	}
}


func TestPostgresTransactionAuditStoreTimestampCollisionUsesAuditIDOrder(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_ordering_collision_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	events := []TransactionAuditEvent{
		{
			ReferenceID:  "audit-order-collision-ref",
			Action:       "STEP_ONE",
			Next:         string(provider.StatusPending),
			ProviderName: "mock",
			Message:      "first",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  "audit-order-collision-ref",
			Action:       "STEP_TWO",
			Previous:     string(provider.StatusPending),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "second",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  "audit-order-collision-ref",
			Action:       "STEP_THREE",
			Previous:     string(provider.StatusSuccess),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "third",
			CreatedAt:    createdAt,
		},
	}
	for i, event := range events {
		if err := store.AppendContext(ctx, event); err != nil {
			t.Fatalf("append event %d: %v", i+1, err)
		}
	}

	got, err := store.AllContextE(ctx, events[0].ReferenceID)
	if err != nil {
		t.Fatalf("read colliding audit timestamps: %v", err)
	}
	if len(got) != len(events) {
		t.Fatalf("expected %d audit events, got %d", len(events), len(got))
	}
	for i, want := range events {
		if got[i] != want {
			t.Fatalf("audit ordering changed at position %d: want=%#v got=%#v", i, want, got[i])
		}
	}
}

func TestPostgresTransactionAuditStoreOrderingSurvivesRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_ordering_restart_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	events := []TransactionAuditEvent{
		{
			ReferenceID:  "audit-ordering-restart-ref",
			Action:       "FIRST",
			Next:         string(provider.StatusPending),
			ProviderName: "mock",
			Message:      "first",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  "audit-ordering-restart-ref",
			Action:       "SECOND",
			Previous:     string(provider.StatusPending),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "second",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  "audit-ordering-restart-ref",
			Action:       "THIRD",
			Previous:     string(provider.StatusSuccess),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "third",
			CreatedAt:    createdAt,
		},
	}
	for i, event := range events {
		if err := store.AppendContext(ctx, event); err != nil {
			t.Fatalf("append event %d: %v", i+1, err)
		}
	}

	beforeRestart, err := store.AllContextE(ctx, events[0].ReferenceID)
	if err != nil {
		t.Fatalf("read audit ordering before restart: %v", err)
	}
	if len(beforeRestart) != len(events) {
		t.Fatalf("expected %d events before restart, got %d", len(events), len(beforeRestart))
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

	restartedStore, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	afterRestart, err := restartedStore.AllContextE(ctx, events[0].ReferenceID)
	if err != nil {
		t.Fatalf("read audit ordering after restart: %v", err)
	}
	if len(afterRestart) != len(beforeRestart) {
		t.Fatalf("audit row count changed after restart: before=%d after=%d", len(beforeRestart), len(afterRestart))
	}
	for i := range beforeRestart {
		if afterRestart[i] != beforeRestart[i] {
			t.Fatalf("audit ordering changed after restart at position %d: before=%#v after=%#v", i, beforeRestart[i], afterRestart[i])
		}
	}
}


func TestPostgresTransactionAuditStoreConcurrentTimestampCollisionPreservesInsertionOrder(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_concurrent_ordering_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	const writers = 16
	start := make(chan struct{})
	done := make(chan error, writers)
	for i := 0; i < writers; i++ {
		i := i
		go func() {
			<-start
			workerCtx, workerCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer workerCancel()

			conn, err := db.Conn(workerCtx)
			if err != nil {
				done <- err
				return
			}
			defer conn.Close()
			if _, err := conn.ExecContext(workerCtx, "SET search_path TO "+schema); err != nil {
				done <- err
				return
			}

			store, err := NewPostgresTransactionAuditStore(conn)
			if err != nil {
				done <- err
				return
			}
			event := TransactionAuditEvent{
				ReferenceID:  "audit-concurrent-ordering-ref",
				Action:       "CONCURRENT_" + strconv.Itoa(i),
				ProviderName: "mock",
				Message:      "same timestamp",
				CreatedAt:    createdAt,
			}
			done <- store.AppendContext(workerCtx, event)
		}()
	}
	close(start)
	for i := 0; i < writers; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent audit append %d: %v", i, err)
		}
	}

	var rows int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+schema+".provider_transaction_audit WHERE reference_id=$1", "audit-concurrent-ordering-ref").Scan(&rows); err != nil {
		t.Fatalf("count concurrent audit rows: %v", err)
	}
	if rows != writers {
		t.Fatalf("expected %d concurrent audit rows, got %d", writers, rows)
	}

	storedOrder, err := func() ([]int, error) {
		rows, err := db.QueryContext(ctx, "SELECT action FROM "+schema+".provider_transaction_audit WHERE reference_id=$1 ORDER BY created_at, audit_id", "audit-concurrent-ordering-ref")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var order []int
		for rows.Next() {
			var action string
			if err := rows.Scan(&action); err != nil {
				return nil, err
			}
			value, err := strconv.Atoi(action[len("CONCURRENT_"):])
			if err != nil {
				return nil, err
			}
			order = append(order, value)
		}
		return order, rows.Err()
	}()
	if err != nil {
		t.Fatalf("read concurrent audit ordering: %v", err)
	}
	if len(storedOrder) != writers {
		t.Fatalf("expected %d ordered rows, got %d", writers, len(storedOrder))
	}
	seen := make(map[int]bool, writers)
	for _, value := range storedOrder {
		if value < 0 || value >= writers {
			t.Fatalf("unexpected concurrent audit action index %d in %#v", value, storedOrder)
		}
		if seen[value] {
			t.Fatalf("duplicate concurrent audit action index %d in %#v", value, storedOrder)
		}
		seen[value] = true
	}
}

func TestPostgresTransactionAuditStoreReadDuringUncommittedAppendSeesOnlyCommittedRows(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_visibility_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	migrationConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin migration connection: %v", err)
	}
	defer migrationConn.Close()
	if _, err := migrationConn.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	if _, err := migrationConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set search path: %v", err)
	}
	applyPostgresMigration(t, migrationConn)

	baseStore, err := NewPostgresTransactionAuditStore(migrationConn)
	if err != nil {
		t.Fatal(err)
	}
	committed := TransactionAuditEvent{
		ReferenceID:  "audit-visibility-ref",
		Action:       "COMMITTED",
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "already committed",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := baseStore.AppendContext(ctx, committed); err != nil {
		t.Fatalf("append committed fixture: %v", err)
	}

	writerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open writer connection: %v", err)
	}
	defer writerConn.Close()
	if _, err := writerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set writer search path: %v", err)
	}
	tx, err := writerConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin append transaction: %v", err)
	}
	defer tx.Rollback()

	pending := TransactionAuditEvent{
		ReferenceID:  "audit-visibility-ref",
		Action:       "UNCOMMITTED",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "visible only after commit",
		CreatedAt:    committed.CreatedAt,
	}
	if _, err := tx.ExecContext(ctx, postgresAuditAppendSQL,
		pending.ReferenceID,
		pending.Action,
		pending.Previous,
		pending.Next,
		pending.ProviderName,
		pending.Message,
		pending.CreatedAt.UTC(),
	); err != nil {
		t.Fatalf("stage uncommitted audit append: %v", err)
	}

	readerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open reader connection: %v", err)
	}
	defer readerConn.Close()
	if _, err := readerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set reader search path: %v", err)
	}
	reader, err := NewPostgresTransactionAuditStore(readerConn)
	if err != nil {
		t.Fatal(err)
	}
	visible, err := reader.AllContextE(ctx, committed.ReferenceID)
	if err != nil {
		t.Fatalf("read during uncommitted append: %v", err)
	}
	if len(visible) != 1 || visible[0] != committed {
		t.Fatalf("reader observed uncommitted or partial audit state: %#v", visible)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit staged audit append: %v", err)
	}
	afterCommit, err := reader.AllContextE(ctx, committed.ReferenceID)
	if err != nil {
		t.Fatalf("read after audit commit: %v", err)
	}
	if len(afterCommit) != 2 || afterCommit[0] != committed || afterCommit[1] != pending {
		t.Fatalf("reader did not observe complete committed audit history: %#v", afterCommit)
	}
}


func TestPostgresTransactionAuditStoreReadVisibilityAfterConcurrentCommit(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_commit_visibility_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	readerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open pinned reader connection: %v", err)
	}
	t.Cleanup(func() { _ = readerConn.Close() })
	if _, err := readerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set pinned reader search path: %v", err)
	}
	store, err := NewPostgresTransactionAuditStore(readerConn)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	first := TransactionAuditEvent{
		ReferenceID:  "audit-commit-visibility-ref",
		Action:       "BEFORE_COMMIT",
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "committed before concurrent writer",
		CreatedAt:    createdAt,
	}
	second := TransactionAuditEvent{
		ReferenceID:  first.ReferenceID,
		Action:       "AFTER_COMMIT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "committed by concurrent writer",
		CreatedAt:    createdAt,
	}
	if err := store.AppendContext(ctx, first); err != nil {
		t.Fatalf("append initial audit event: %v", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open concurrent writer connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set concurrent writer search path: %v", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin concurrent writer transaction: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, postgresAuditAppendSQL,
		second.ReferenceID,
		second.Action,
		second.Previous,
		second.Next,
		second.ProviderName,
		second.Message,
		second.CreatedAt.UTC(),
	); err != nil {
		t.Fatalf("stage concurrent audit append: %v", err)
	}

	beforeCommit, err := store.AllContextE(ctx, first.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history before concurrent commit: %v", err)
	}
	if len(beforeCommit) != 1 || beforeCommit[0] != first {
		t.Fatalf("reader must see only committed audit history before commit, got %#v", beforeCommit)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit concurrent audit append: %v", err)
	}

	afterCommit, err := store.AllContextE(ctx, first.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history after concurrent commit: %v", err)
	}
	if len(afterCommit) != 2 {
		t.Fatalf("reader must converge to complete committed audit history, got %d rows", len(afterCommit))
	}
	if afterCommit[0] != first || afterCommit[1] != second {
		t.Fatalf("committed audit ordering changed after concurrent commit: %#v", afterCommit)
	}
}


func TestPostgresTransactionAuditStoreRepeatedConcurrentSnapshotsConverge(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_snapshot_convergence_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	readerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open pinned reader connection: %v", err)
	}
	defer readerConn.Close()
	if _, err := readerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set reader search path: %v", err)
	}
	reader, err := NewPostgresTransactionAuditStore(readerConn)
	if err != nil {
		t.Fatal(err)
	}

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	const writers = 8
	committed := make([]TransactionAuditEvent, 0, writers)

	for i := 0; i < writers; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("open writer connection %d: %v", i, err)
		}
		if _, err := conn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
			conn.Close()
			t.Fatalf("set writer search path %d: %v", i, err)
		}
		store, err := NewPostgresTransactionAuditStore(conn)
		if err != nil {
			conn.Close()
			t.Fatal(err)
		}
		event := TransactionAuditEvent{
			ReferenceID:  "audit-snapshot-convergence-ref",
			Action:       "COMMITTED_" + strconv.Itoa(i),
			ProviderName: "mock",
			Message:      "monotonic snapshot",
			CreatedAt:    createdAt,
		}
		if err := store.AppendContext(ctx, event); err != nil {
			conn.Close()
			t.Fatalf("append committed event %d: %v", i, err)
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("close writer connection %d: %v", i, err)
		}
		committed = append(committed, event)

		snapshot, err := reader.AllContextE(ctx, event.ReferenceID)
		if err != nil {
			t.Fatalf("read snapshot after commit %d: %v", i, err)
		}
		if len(snapshot) != i+1 {
			t.Fatalf("expected monotonic snapshot size %d after commit %d, got %d", i+1, i, len(snapshot))
		}
		for j, want := range committed {
			if snapshot[j] != want {
				t.Fatalf("snapshot after commit %d changed at position %d: want=%#v got=%#v", i, j, want, snapshot[j])
			}
		}
	}

	for i := 0; i < 4; i++ {
		snapshot, err := reader.AllContextE(ctx, "audit-snapshot-convergence-ref")
		if err != nil {
			t.Fatalf("repeated final snapshot %d: %v", i, err)
		}
		if len(snapshot) != writers {
			t.Fatalf("final snapshot %d must contain all %d committed events, got %d", i, writers, len(snapshot))
		}
		for j, want := range committed {
			if snapshot[j] != want {
				t.Fatalf("repeated final snapshot %d changed at position %d: want=%#v got=%#v", i, j, want, snapshot[j])
			}
		}
	}
}


func TestPostgresTransactionAuditStoreSnapshotOrderingAfterReaderRecovery(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_snapshot_recovery_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	events := []TransactionAuditEvent{
		{
			ReferenceID:  "audit-snapshot-recovery-ref",
			Action:       "SNAPSHOT_ONE",
			Next:         string(provider.StatusPending),
			ProviderName: "mock",
			Message:      "first snapshot event",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  "audit-snapshot-recovery-ref",
			Action:       "SNAPSHOT_TWO",
			Previous:     string(provider.StatusPending),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "second snapshot event",
			CreatedAt:    createdAt,
		},
	}
	for i, event := range events {
		if err := store.AppendContext(ctx, event); err != nil {
			t.Fatalf("append event %d: %v", i+1, err)
		}
	}

	beforeOne, err := store.AllContextE(ctx, events[0].ReferenceID)
	if err != nil {
		t.Fatalf("read first snapshot before recovery: %v", err)
	}
	beforeTwo, err := store.AllContextE(ctx, events[0].ReferenceID)
	if err != nil {
		t.Fatalf("read second snapshot before recovery: %v", err)
	}
	if len(beforeOne) != 2 || len(beforeTwo) != 2 {
		t.Fatalf("expected two events in pre-recovery snapshots, got first=%d second=%d", len(beforeOne), len(beforeTwo))
	}
	for i := range events {
		if beforeOne[i] != events[i] || beforeTwo[i] != events[i] {
			t.Fatalf("pre-recovery snapshot ordering changed: first=%#v second=%#v", beforeOne, beforeTwo)
		}
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
		t.Fatalf("restore search path after reader recovery: %v", err)
	}
	recoveredStore, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}

	afterOne, err := recoveredStore.AllContextE(ctx, events[0].ReferenceID)
	if err != nil {
		t.Fatalf("read first snapshot after recovery: %v", err)
	}
	afterTwo, err := recoveredStore.AllContextE(ctx, events[0].ReferenceID)
	if err != nil {
		t.Fatalf("read second snapshot after recovery: %v", err)
	}
	if len(afterOne) != 2 || len(afterTwo) != 2 {
		t.Fatalf("expected two events in recovered snapshots, got first=%d second=%d", len(afterOne), len(afterTwo))
	}
	for i := range events {
		if afterOne[i] != events[i] || afterTwo[i] != events[i] {
			t.Fatalf("post-recovery snapshot ordering changed: first=%#v second=%#v", afterOne, afterTwo)
		}
	}
	if len(afterOne) != len(beforeOne) || len(afterTwo) != len(beforeTwo) {
		t.Fatalf("snapshot cardinality changed across reader recovery: before=%d/%d after=%d/%d", len(beforeOne), len(beforeTwo), len(afterOne), len(afterTwo))
	}
}


func TestPostgresTransactionAuditStoreAmbiguousWriterCommitRecovery(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_ambiguous_commit_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	writerDB, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("open writer postgres: %v", err)
	}
	defer writerDB.Close()
	if err := writerDB.PingContext(ctx); err != nil {
		t.Fatalf("ping writer postgres: %v", err)
	}
	if _, err := writerDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set writer search path: %v", err)
	}

	writerStore, err := NewPostgresTransactionAuditStore(writerDB)
	if err != nil {
		t.Fatal(err)
	}
	event := TransactionAuditEvent{
		ReferenceID:  "audit-ambiguous-commit-ref",
		Action:       "COMMIT_BEFORE_CONNECTION_LOSS",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "commit acknowledged before connection loss",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	if err := writerStore.AppendContext(ctx, event); err != nil {
		t.Fatalf("append committed audit event: %v", err)
	}

	readerStore, err := NewPostgresTransactionAuditStore(writerDB)
	if err != nil {
		t.Fatal(err)
	}
	beforeFailure, err := readerStore.AllContextE(ctx, event.ReferenceID)
	if err != nil {
		t.Fatalf("read committed audit event before simulated transport loss: %v", err)
	}
	if len(beforeFailure) != 1 || beforeFailure[0] != event {
		t.Fatalf("expected exactly one committed audit event before writer loss, got %#v", beforeFailure)
	}

	if err := writerDB.Close(); err != nil {
		t.Fatalf("close writer after commit: %v", err)
	}

	reopened, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("reopen postgres after ambiguous writer failure: %v", err)
	}
	defer reopened.Close()
	if err := reopened.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened postgres: %v", err)
	}
	if _, err := reopened.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("restore search path after ambiguous writer failure: %v", err)
	}

	recoveredStore, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := recoveredStore.AllContextE(ctx, event.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history after ambiguous writer recovery: %v", err)
	}
	if len(recovered) != 1 || recovered[0] != event {
		t.Fatalf("recovered audit history must contain exactly the committed event once, got %#v", recovered)
	}

	var count int
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_transaction_audit WHERE reference_id=$1", event.ReferenceID).Scan(&count); err != nil {
		t.Fatalf("count recovered audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("ambiguous writer recovery must not create a duplicate audit row, got %d", count)
	}
}


func TestPostgresTransactionAuditStoreSnapshotStabilityUnderWriterRecovery(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_writer_recovery_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	readerStore, err := NewPostgresTransactionAuditStore(db)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	initial := TransactionAuditEvent{
		ReferenceID:  "audit-writer-recovery-ref",
		Action:       "INITIAL",
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "initial committed evidence",
		CreatedAt:    createdAt,
	}
	if err := readerStore.AppendContext(ctx, initial); err != nil {
		t.Fatalf("append initial audit event: %v", err)
	}

	before, err := readerStore.AllContextE(ctx, initial.ReferenceID)
	if err != nil {
		t.Fatalf("read initial snapshot: %v", err)
	}
	if len(before) != 1 || before[0] != initial {
		t.Fatalf("unexpected initial audit snapshot: %#v", before)
	}

	writerDB, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("open writer postgres: %v", err)
	}
	writerAlive := true
	defer func() {
		if writerAlive {
			_ = writerDB.Close()
		}
	}()

	if err := writerDB.PingContext(ctx); err != nil {
		t.Fatalf("ping writer postgres: %v", err)
	}
	if _, err := writerDB.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set writer search path: %v", err)
	}
	writerStore, err := NewPostgresTransactionAuditStore(writerDB)
	if err != nil {
		t.Fatal(err)
	}

	writerEvent := TransactionAuditEvent{
		ReferenceID:  initial.ReferenceID,
		Action:       "RECOVERED_WRITER_APPEND",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "writer recovered before append",
		CreatedAt:    createdAt,
	}

	if err := writerDB.Close(); err != nil {
		t.Fatalf("close writer connection pool: %v", err)
	}
	writerAlive = false

	if err := writerStore.AppendContext(context.Background(), writerEvent); err == nil {
		t.Fatal("expected append failure while writer connection is closed")
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("writer recovery failure must not be classified as context termination: %v", err)
	}

	unchanged, err := readerStore.AllContextE(ctx, initial.ReferenceID)
	if err != nil {
		t.Fatalf("read snapshot after failed writer append: %v", err)
	}
	if len(unchanged) != 1 || unchanged[0] != initial {
		t.Fatalf("failed writer append must not alter committed audit snapshot: %#v", unchanged)
	}

	reopened, err := sql.Open("pgx", os.Getenv("DESKAPROVIDER_POSTGRES_DSN"))
	if err != nil {
		t.Fatalf("reopen writer postgres: %v", err)
	}
	defer reopened.Close()
	if err := reopened.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened writer postgres: %v", err)
	}
	if _, err := reopened.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("restore writer search path after recovery: %v", err)
	}
	recoveredWriter, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	if err := recoveredWriter.AppendContext(ctx, writerEvent); err != nil {
		t.Fatalf("append after writer recovery: %v", err)
	}

	final, err := readerStore.AllContextE(ctx, initial.ReferenceID)
	if err != nil {
		t.Fatalf("read final audit snapshot after writer recovery: %v", err)
	}
	if len(final) != 2 || final[0] != initial || final[1] != writerEvent {
		t.Fatalf("final audit snapshot changed unexpectedly after writer recovery: %#v", final)
	}

	secondFinal, err := readerStore.AllContextE(ctx, initial.ReferenceID)
	if err != nil {
		t.Fatalf("repeat final audit snapshot read: %v", err)
	}
	if len(secondFinal) != len(final) || secondFinal[0] != final[0] || secondFinal[1] != final[1] {
		t.Fatalf("repeated final audit snapshots did not converge: first=%#v second=%#v", final, secondFinal)
	}

	var count int
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_transaction_audit WHERE reference_id=$1", initial.ReferenceID).Scan(&count); err != nil {
		t.Fatalf("count writer recovery audit rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("writer recovery should produce exactly two committed audit rows, got %d", count)
	}
}


func TestPostgresTransactionAuditStoreSnapshotRecoveryAfterConnectionRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_snapshot_restart_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	expected := []TransactionAuditEvent{
		{
			ReferenceID:  "audit-snapshot-restart-ref",
			Action:       "SNAPSHOT_PENDING",
			Next:         string(provider.StatusPending),
			ProviderName: "mock",
			Message:      "pending",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  "audit-snapshot-restart-ref",
			Action:       "SNAPSHOT_RESULT",
			Previous:     string(provider.StatusPending),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "success",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  "audit-snapshot-restart-ref",
			Action:       "SNAPSHOT_RECONCILED",
			Previous:     string(provider.StatusSuccess),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "reconciled",
			CreatedAt:    createdAt.Add(time.Microsecond),
		},
	}
	for i, event := range expected {
		if err := store.AppendContext(ctx, event); err != nil {
			t.Fatalf("append snapshot event %d: %v", i+1, err)
		}
	}

	beforeRestart, err := store.AllContextE(ctx, expected[0].ReferenceID)
	if err != nil {
		t.Fatalf("read audit snapshot before connection restart: %v", err)
	}
	if len(beforeRestart) != len(expected) {
		t.Fatalf("expected %d events before connection restart, got %d", len(expected), len(beforeRestart))
	}
	for i, want := range expected {
		if beforeRestart[i] != want {
			t.Fatalf("pre-restart audit snapshot changed at %d: want=%#v got=%#v", i, want, beforeRestart[i])
		}
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
		t.Fatalf("restore search path after connection restart: %v", err)
	}

	recoveredStore, err := NewPostgresTransactionAuditStore(reopened)
	if err != nil {
		t.Fatal(err)
	}
	afterRestart, err := recoveredStore.AllContextE(ctx, expected[0].ReferenceID)
	if err != nil {
		t.Fatalf("read audit snapshot after connection restart: %v", err)
	}
	if len(afterRestart) != len(expected) {
		t.Fatalf("expected %d events after connection restart, got %d", len(expected), len(afterRestart))
	}
	for i, want := range expected {
		if afterRestart[i] != want {
			t.Fatalf("post-restart audit snapshot changed at %d: want=%#v got=%#v", i, want, afterRestart[i])
		}
	}
}

func TestPostgresTransactionAuditStoreSnapshotRecoveryCrossChecksTransactionState(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_snapshot_transaction_crosscheck_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "success",
		PurchaseStatus: provider.StatusSuccess,
		Price:          20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	ops := operational.NewMemoryStore()
	if err := ops.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance:      100000,
		Health:       operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, ops, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: postgresIntegrationReference(),
		Amount:      20000,
	}
	service, err := NewServiceWithStoreContextAndAudit(ctx, router, transactionStore, auditStore)
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}
	execution, err := service.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("initial purchase: %v", err)
	}
	if execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected terminal success, got %q", execution.Result.Status)
	}
	if mock.PurchaseCount(req.ReferenceID) != 1 {
		t.Fatalf("expected exactly one provider submission, got %d", mock.PurchaseCount(req.ReferenceID))
	}

	beforeRestart, ok, err := transactionStore.GetContextE(ctx, req.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read durable transaction before restart: ok=%v err=%v", ok, err)
	}
	beforeAudit, err := auditStore.AllContextE(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("read audit snapshot before restart: %v", err)
	}
	if len(beforeAudit) != 2 {
		t.Fatalf("expected two audit lifecycle events before restart, got %d", len(beforeAudit))
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

	afterRestart, ok, err := restartedTransactionStore.GetContextE(ctx, req.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read durable transaction after restart: ok=%v err=%v", ok, err)
	}
	if afterRestart.Request != beforeRestart.Request || afterRestart.Execution != beforeRestart.Execution || afterRestart.Version != beforeRestart.Version {
		t.Fatalf("transaction state changed across restart: before=%#v after=%#v", beforeRestart, afterRestart)
	}

	auditAfterRestart, err := restartedAuditStore.AllContextE(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("read audit snapshot after restart: %v", err)
	}
	if len(auditAfterRestart) != len(beforeAudit) {
		t.Fatalf("audit snapshot changed length after restart: before=%d after=%d", len(beforeAudit), len(auditAfterRestart))
	}
	for i := range beforeAudit {
		if auditAfterRestart[i] != beforeAudit[i] {
			t.Fatalf("audit snapshot changed at %d: before=%#v after=%#v", i, beforeAudit[i], auditAfterRestart[i])
		}
	}

	secondExecution, err := service.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("repeat purchase after recovered persistence: %v", err)
	}
	if secondExecution.Result.Status != provider.StatusSuccess {
		t.Fatalf("repeat purchase changed terminal result, got %q", secondExecution.Result.Status)
	}
	if mock.PurchaseCount(req.ReferenceID) != 1 {
		t.Fatalf("recovered transaction state must prevent a second provider submission, got %d", mock.PurchaseCount(req.ReferenceID))
	}
}


func TestPostgresTransactionAuditStoreConflictingAuditDoesNotOverrideTransactionState(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_conflict_authority_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "success",
		PurchaseStatus: provider.StatusSuccess,
		Price:          20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	ops := operational.NewMemoryStore()
	if err := ops.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance:      100000,
		Health:       operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, ops, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "audit-conflict-authority-ref",
		Amount:      20000,
	}
	service, err := NewServiceWithStoreContextAndAudit(ctx, router, transactionStore, auditStore)
	if err != nil {
		t.Fatal(err)
	}
	execution, err := service.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("initial purchase: %v", err)
	}
	if execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected durable terminal success, got %q", execution.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("expected one provider submission, got %d", got)
	}

	// Inject audit evidence that conflicts with the authoritative terminal transaction.
	conflictingAudit := TransactionAuditEvent{
		ReferenceID:  req.ReferenceID,
		Action:       "CONFLICTING_OBSERVATION",
		Previous:     string(provider.StatusSuccess),
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "conflicting audit evidence must remain observational",
		CreatedAt:    time.Now().UTC().Add(time.Second).Truncate(time.Microsecond),
	}
	if err := auditStore.AppendContext(ctx, conflictingAudit); err != nil {
		t.Fatalf("append conflicting audit evidence: %v", err)
	}

	events, err := auditStore.AllContextE(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("read conflicting audit history: %v", err)
	}
	if len(events) != 3 || events[2] != conflictingAudit {
		t.Fatalf("expected conflicting audit evidence to remain durably observable, got %#v", events)
	}

	persisted, ok, readErr := transactionStore.GetContextE(ctx, req.ReferenceID)
	if readErr != nil || !ok {
		t.Fatalf("read authoritative transaction state: ok=%v err=%v", ok, readErr)
	}
	if persisted.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("conflicting audit evidence must not override terminal transaction state, got %q", persisted.Execution.Result.Status)
	}

	retry, retryErr := service.Purchase(ctx, req)
	if retryErr != nil {
		t.Fatalf("repeated purchase must follow durable transaction state, got %v", retryErr)
	}
	if retry.Result.Status != provider.StatusSuccess {
		t.Fatalf("repeated purchase must return durable terminal result, got %q", retry.Result.Status)
	}
	if got := mock.PurchaseCount(req.ReferenceID); got != 1 {
		t.Fatalf("conflicting audit evidence must not authorize provider resubmission, got %d", got)
	}
}


func TestPostgresTransactionAuditStoreConflictingAuditRemainsNonAuthoritativeAfterRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_conflict_restart_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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
		Products:       []provider.Product{{Code: "pln20", Name: "PLN 20"}},
		ProviderCode:   "00",
		Message:        "success",
		PurchaseStatus: provider.StatusSuccess,
		Price:          20000,
	})
	if err := registry.Register("mock", mock); err != nil {
		t.Fatal(err)
	}
	ops := operational.NewMemoryStore()
	if err := ops.Put(operational.Snapshot{
		ProviderName: "mock",
		Balance:      100000,
		Health:       operational.HealthHealthy,
	}); err != nil {
		t.Fatal(err)
	}
	router, err := New(registry, ops, map[string]int{"mock": 1})
	if err != nil {
		t.Fatal(err)
	}

	req := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "audit-conflict-restart-ref",
		Amount:      20000,
	}
	service, err := NewServiceWithStoreContextAndAudit(ctx, router, transactionStore, auditStore)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := service.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("initial purchase: %v", err)
	}
	if initial.Result.Status != provider.StatusSuccess {
		t.Fatalf("expected terminal success, got %q", initial.Result.Status)
	}
	if mock.PurchaseCount(req.ReferenceID) != 1 {
		t.Fatalf("expected one provider submission, got %d", mock.PurchaseCount(req.ReferenceID))
	}

	conflictingAudit := TransactionAuditEvent{
		ReferenceID:  req.ReferenceID,
		Action:       "RESTART_CONFLICTING_OBSERVATION",
		Previous:     string(provider.StatusSuccess),
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "conflicting audit must remain observational after restart",
		CreatedAt:    time.Now().UTC().Add(time.Second).Truncate(time.Microsecond),
	}
	if err := auditStore.AppendContext(ctx, conflictingAudit); err != nil {
		t.Fatalf("append conflicting audit evidence: %v", err)
	}

	beforeRestart, err := auditStore.AllContextE(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("read conflicting audit history before restart: %v", err)
	}
	if len(beforeRestart) != 3 || beforeRestart[2] != conflictingAudit {
		t.Fatalf("unexpected audit history before restart: %#v", beforeRestart)
	}
	beforeTransaction, ok, err := transactionStore.GetContextE(ctx, req.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read transaction state before restart: ok=%v err=%v", ok, err)
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
	restartedService, err := NewServiceWithStoreContextAndAudit(ctx, router, restartedTransactionStore, restartedAuditStore)
	if err != nil {
		t.Fatalf("reconstruct service after restart: %v", err)
	}

	afterRestart, err := restartedAuditStore.AllContextE(ctx, req.ReferenceID)
	if err != nil {
		t.Fatalf("read conflicting audit history after restart: %v", err)
	}
	if len(afterRestart) != len(beforeRestart) {
		t.Fatalf("audit history length changed after restart: before=%d after=%d", len(beforeRestart), len(afterRestart))
	}
	for i := range beforeRestart {
		if afterRestart[i] != beforeRestart[i] {
			t.Fatalf("audit history changed at %d after restart: before=%#v after=%#v", i, beforeRestart[i], afterRestart[i])
		}
	}

	afterTransaction, ok, err := restartedTransactionStore.GetContextE(ctx, req.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read transaction state after restart: ok=%v err=%v", ok, err)
	}
	if afterTransaction != beforeTransaction {
		t.Fatalf("durable transaction state changed after restart: before=%#v after=%#v", beforeTransaction, afterTransaction)
	}
	if afterTransaction.Execution.Result.Status != provider.StatusSuccess {
		t.Fatalf("conflicting audit evidence must not override terminal transaction state after restart, got %q", afterTransaction.Execution.Result.Status)
	}

	retry, err := restartedService.Purchase(ctx, req)
	if err != nil {
		t.Fatalf("repeat purchase after restart must follow durable transaction state: %v", err)
	}
	if retry.Result.Status != provider.StatusSuccess {
		t.Fatalf("repeat purchase changed terminal result after restart, got %q", retry.Result.Status)
	}
	if mock.PurchaseCount(req.ReferenceID) != 1 {
		t.Fatalf("conflicting audit evidence must not authorize resubmission after restart, got %d", mock.PurchaseCount(req.ReferenceID))
	}
}


func TestPostgresTransactionAuditStoreConflictWithConcurrentTransactionUpdateRemainsNonAuthoritative(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_conflict_concurrent_tx_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	migrationConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open migration connection: %v", err)
	}
	defer migrationConn.Close()
	if _, err := migrationConn.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	if _, err := migrationConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set migration search path: %v", err)
	}
	applyPostgresMigration(t, migrationConn)

	state := postgresPendingState()
	state.Request.ReferenceID = "audit-conflict-concurrent-tx-ref"
	state.Execution.Result.ReferenceID = state.Request.ReferenceID
	state.Execution.Result.Status = provider.StatusPending
	state.Execution.Result.ProviderCode = ""
	state.Execution.Result.Message = "pending"
	state.Execution.Result.SerialNumber = ""
	state.Request.Amount = 20000
	state.Execution.Result.Price = state.Request.Amount
	state.Version = 1

	seedConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open seed connection: %v", err)
	}
	defer seedConn.Close()
	if _, err := seedConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set seed search path: %v", err)
	}
	seedStore, err := NewPostgresTransactionStore(seedConn)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedStore.PutContext(ctx, state); err != nil {
		t.Fatalf("persist pending transaction: %v", err)
	}

	transitionConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open transaction connection: %v", err)
	}
	defer transitionConn.Close()
	if _, err := transitionConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set transition search path: %v", err)
	}
	transitionTx, err := transitionConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin durable transaction update: %v", err)
	}
	defer transitionTx.Rollback()

	next := state
	next.Execution.Result.Status = provider.StatusSuccess
	next.Execution.Result.ProviderCode = "00"
	next.Execution.Result.Message = "success"
	next.Execution.Result.SerialNumber = "SN-CONCURRENT"
	if _, err := transitionTx.ExecContext(ctx, `UPDATE provider_transactions
SET status=$1, provider_code=$2, message=$3, serial_number=$4, price=$5, version=version+1
WHERE reference_id=$6 AND status='pending' AND version=$7`,
		string(next.Execution.Result.Status),
		next.Execution.Result.ProviderCode,
		next.Execution.Result.Message,
		next.Execution.Result.SerialNumber,
		next.Execution.Result.Price,
		next.Request.ReferenceID,
		state.Version,
	); err != nil {
		t.Fatalf("stage durable transaction update: %v", err)
	}

	auditConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open audit connection: %v", err)
	}
	defer auditConn.Close()
	if _, err := auditConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set audit search path: %v", err)
	}
	auditTx, err := auditConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin contradictory audit transaction: %v", err)
	}
	defer auditTx.Rollback()

	conflict := TransactionAuditEvent{
		ReferenceID:  state.Request.ReferenceID,
		Action:       "CONTRADICTORY_AUDIT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusFailed),
		ProviderName: "mock",
		Message:      "audit says failed while transaction moves to success",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
	if _, err := auditTx.ExecContext(ctx, postgresAuditAppendSQL,
		conflict.ReferenceID,
		conflict.Action,
		conflict.Previous,
		conflict.Next,
		conflict.ProviderName,
		conflict.Message,
		conflict.CreatedAt.UTC(),
	); err != nil {
		t.Fatalf("stage contradictory audit event: %v", err)
	}

	readerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open reader connection: %v", err)
	}
	defer readerConn.Close()
	if _, err := readerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set reader search path: %v", err)
	}
	readerStore, err := NewPostgresTransactionStore(readerConn)
	if err != nil {
		t.Fatal(err)
	}
	beforeCommit, ok, err := readerStore.GetContextE(ctx, state.Request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read transaction before concurrent commits: ok=%v err=%v", ok, err)
	}
	if beforeCommit.Version != 1 || beforeCommit.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("transaction state must remain pending before writer commit, got %#v", beforeCommit)
	}
	readerAuditStore, err := NewPostgresTransactionAuditStore(readerConn)
	if err != nil {
		t.Fatal(err)
	}
	beforeAudit, err := readerAuditStore.AllContextE(ctx, state.Request.ReferenceID)
	if err != nil {
		t.Fatalf("read audit before concurrent commits: %v", err)
	}
	if len(beforeAudit) != 0 {
		t.Fatalf("uncommitted contradictory audit event must remain invisible, got %#v", beforeAudit)
	}

	if err := auditTx.Commit(); err != nil {
		t.Fatalf("commit contradictory audit event: %v", err)
	}
	if err := transitionTx.Commit(); err != nil {
		t.Fatalf("commit durable transaction update: %v", err)
	}

	postCommitReaderConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open post-commit reader connection: %v", err)
	}
	defer postCommitReaderConn.Close()
	if _, err := postCommitReaderConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set post-commit reader search path: %v", err)
	}
	postCommitReaderStore, err := NewPostgresTransactionStore(postCommitReaderConn)
	if err != nil {
		t.Fatal(err)
	}
	persisted, ok, readErr := postCommitReaderStore.GetContextE(ctx, state.Request.ReferenceID)
	if readErr != nil || !ok {
		t.Fatalf("read committed transaction update: ok=%v err=%v", ok, readErr)
	}
	if persisted.Execution.Result.Status != provider.StatusSuccess ||
		persisted.Execution.Result.ProviderCode != "00" ||
		persisted.Execution.Result.Message != "success" ||
		persisted.Execution.Result.SerialNumber != "SN-CONCURRENT" {
		t.Fatalf("durable transaction state must remain authoritative, got %#v", persisted.Execution.Result)
	}

	events, err := readerAuditStore.AllContextE(ctx, state.Request.ReferenceID)
	if err != nil {
		t.Fatalf("read committed contradictory audit evidence: %v", err)
	}
	if len(events) != 1 || events[0] != conflict {
		t.Fatalf("expected contradictory audit evidence to remain observable, got %#v", events)
	}
}

func TestPostgresTransactionAuditAndTransactionCommitOrderingRemainsNonAuthoritative(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_tx_commit_order_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	migrationConn, err := db.Conn(ctx)
	if err != nil { t.Fatalf("open migration connection: %v", err) }
	defer migrationConn.Close()
	if _, err := migrationConn.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatalf("create isolated schema: %v", err) }
	t.Cleanup(func() { _, _ = db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	if _, err := migrationConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatalf("set migration search path: %v", err) }
	applyPostgresMigration(t, migrationConn)

	state := postgresPendingState()
	state.Request.ReferenceID = "audit-tx-commit-order-ref"
	state.Request.Amount = 20000
	state.Execution.Result.ReferenceID = state.Request.ReferenceID
	state.Execution.Result.Price = state.Request.Amount
	state.Version = 1

	seedConn, err := db.Conn(ctx)
	if err != nil { t.Fatalf("open seed connection: %v", err) }
	defer seedConn.Close()
	if _, err := seedConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatalf("set seed search path: %v", err) }
	seedStore, err := NewPostgresTransactionStore(seedConn)
	if err != nil { t.Fatal(err) }
	if err := seedStore.PutContext(ctx, state); err != nil { t.Fatalf("persist pending transaction: %v", err) }

	transitionConn, err := db.Conn(ctx)
	if err != nil { t.Fatalf("open transaction writer connection: %v", err) }
	defer transitionConn.Close()
	if _, err := transitionConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatalf("set transaction writer search path: %v", err) }
	transitionTx, err := transitionConn.BeginTx(ctx, nil)
	if err != nil { t.Fatalf("begin transaction writer: %v", err) }
	defer transitionTx.Rollback()
	transactionUpdateSQL := "UPDATE provider_transactions SET status='success', provider_code='00', message='success', serial_number='SN-COMMIT-ORDER', price=20000, version=version+1 WHERE reference_id=$1 AND status='pending' AND version=1"
	if _, err := transitionTx.ExecContext(ctx, transactionUpdateSQL, state.Request.ReferenceID); err != nil { t.Fatalf("stage durable transaction success: %v", err) }

	auditConn, err := db.Conn(ctx)
	if err != nil { t.Fatalf("open audit writer connection: %v", err) }
	defer auditConn.Close()
	if _, err := auditConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatalf("set audit writer search path: %v", err) }
	auditTx, err := auditConn.BeginTx(ctx, nil)
	if err != nil { t.Fatalf("begin audit writer: %v", err) }
	defer auditTx.Rollback()

	auditEvent := TransactionAuditEvent{ReferenceID: state.Request.ReferenceID, Action: "PURCHASE_RESULT", Previous: string(provider.StatusPending), Next: string(provider.StatusSuccess), ProviderName: "mock", Message: "commit-order-evidence", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if _, err := auditTx.ExecContext(ctx, postgresAuditAppendSQL, auditEvent.ReferenceID, auditEvent.Action, auditEvent.Previous, auditEvent.Next, auditEvent.ProviderName, auditEvent.Message, auditEvent.CreatedAt.UTC()); err != nil { t.Fatalf("stage audit evidence: %v", err) }

	beforeConn, err := db.Conn(ctx)
	if err != nil { t.Fatalf("open pre-commit reader connection: %v", err) }
	defer beforeConn.Close()
	if _, err := beforeConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatalf("set pre-commit reader search path: %v", err) }
	beforeTxStore, err := NewPostgresTransactionStore(beforeConn)
	if err != nil { t.Fatal(err) }
	beforeAuditStore, err := NewPostgresTransactionAuditStore(beforeConn)
	if err != nil { t.Fatal(err) }
	beforeState, ok, err := beforeTxStore.GetContextE(ctx, state.Request.ReferenceID)
	if err != nil || !ok || beforeState.Version != 1 || beforeState.Execution.Result.Status != provider.StatusPending { t.Fatalf("before commit must expose only durable pending transaction: state=%#v ok=%v err=%v", beforeState, ok, err) }
	beforeEvents, err := beforeAuditStore.AllContextE(ctx, state.Request.ReferenceID)
	if err != nil { t.Fatalf("read audit before commits: %v", err) }
	if len(beforeEvents) != 0 { t.Fatalf("uncommitted audit evidence must remain invisible before commit, got %#v", beforeEvents) }

	if err := transitionTx.Commit(); err != nil { t.Fatalf("commit durable transaction first: %v", err) }

	afterTxFirstConn, err := db.Conn(ctx)
	if err != nil { t.Fatalf("open transaction-first reader: %v", err) }
	defer afterTxFirstConn.Close()
	if _, err := afterTxFirstConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatalf("set transaction-first reader search path: %v", err) }
	afterTxFirstStore, err := NewPostgresTransactionStore(afterTxFirstConn)
	if err != nil { t.Fatal(err) }
	afterTxFirstAudit, err := NewPostgresTransactionAuditStore(afterTxFirstConn)
	if err != nil { t.Fatal(err) }
	transactionFirst, ok, err := afterTxFirstStore.GetContextE(ctx, state.Request.ReferenceID)
	if err != nil || !ok || transactionFirst.Version != 2 || transactionFirst.Execution.Result.Status != provider.StatusSuccess { t.Fatalf("transaction-first commit must expose terminal transaction state: state=%#v ok=%v err=%v", transactionFirst, ok, err) }
	auditFirst, err := afterTxFirstAudit.AllContextE(ctx, state.Request.ReferenceID)
	if err != nil { t.Fatalf("read audit after transaction-first commit: %v", err) }
	if len(auditFirst) != 0 { t.Fatalf("audit must remain invisible while its transaction is uncommitted, got %#v", auditFirst) }

	if err := auditTx.Commit(); err != nil { t.Fatalf("commit audit after transaction: %v", err) }

	afterAuditConn, err := db.Conn(ctx)
	if err != nil { t.Fatalf("open post-audit reader: %v", err) }
	defer afterAuditConn.Close()
	if _, err := afterAuditConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil { t.Fatalf("set post-audit reader search path: %v", err) }
	afterAuditTxStore, err := NewPostgresTransactionStore(afterAuditConn)
	if err != nil { t.Fatal(err) }
	afterAuditStore, err := NewPostgresTransactionAuditStore(afterAuditConn)
	if err != nil { t.Fatal(err) }
	finalState, ok, err := afterAuditTxStore.GetContextE(ctx, state.Request.ReferenceID)
	if err != nil || !ok || finalState.Version != 2 || finalState.Execution.Result.Status != provider.StatusSuccess { t.Fatalf("audit commit must not alter terminal transaction state: state=%#v ok=%v err=%v", finalState, ok, err) }
	finalAudit, err := afterAuditStore.AllContextE(ctx, state.Request.ReferenceID)
	if err != nil { t.Fatalf("read final audit history: %v", err) }
	if len(finalAudit) != 1 || finalAudit[0] != auditEvent { t.Fatalf("expected one committed audit event after second commit, got %#v", finalAudit) }
}

func TestPostgresAuditFirstCommitDoesNotAuthorizeTransactionTransition(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_first_commit_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	auditReaderConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open audit reader connection: %v", err)
	}
	defer auditReaderConn.Close()
	if _, err := auditReaderConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set audit reader search path: %v", err)
	}
	auditStore, err := NewPostgresTransactionAuditStore(auditReaderConn)
	if err != nil {
		t.Fatal(err)
	}
	transactionReaderConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open transaction reader connection: %v", err)
	}
	defer transactionReaderConn.Close()
	if _, err := transactionReaderConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set transaction reader search path: %v", err)
	}
	transactionStore, err := NewPostgresTransactionStore(transactionReaderConn)
	if err != nil {
		t.Fatal(err)
	}

	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "audit-first-commit-ref",
		Amount:      20000,
	}
	event := TransactionAuditEvent{
		ReferenceID:  request.ReferenceID,
		Action:       "PURCHASE_RESULT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "audit committed before transaction",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open audit-first connection: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set audit-first search path: %v", err)
	}
	auditTx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin audit transaction: %v", err)
	}
	if _, err := auditTx.ExecContext(ctx, postgresAuditAppendSQL,
		event.ReferenceID,
		event.Action,
		event.Previous,
		event.Next,
		event.ProviderName,
		event.Message,
		event.CreatedAt.UTC(),
	); err != nil {
		t.Fatalf("stage audit event: %v", err)
	}
	if err := auditTx.Commit(); err != nil {
		t.Fatalf("commit audit event first: %v", err)
	}

	visibleAudit, err := auditStore.AllContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history after audit-first commit: %v", err)
	}
	if len(visibleAudit) != 1 || visibleAudit[0] != event {
		t.Fatalf("expected committed audit evidence to be visible first, got %#v", visibleAudit)
	}

	if _, ok, err := transactionStore.GetContextE(ctx, request.ReferenceID); err != nil {
		t.Fatalf("read transaction state before transaction commit: %v", err)
	} else if ok {
		t.Fatal("audit-first commit must not create transaction authority state")
	}

	pending := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: event.ProviderName,
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				CustomerNo:  request.CustomerNo,
				ProductCode: request.ProductCode,
				Status:      provider.StatusPending,
			},
		},
		Version: 1,
	}
	if err := transactionStore.PutContext(ctx, pending); err != nil {
		t.Fatalf("commit pending transaction state after audit: %v", err)
	}

	committed, ok, err := transactionStore.GetContextE(ctx, request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read committed transaction state: ok=%v err=%v", ok, err)
	}
	if committed.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("expected transaction to remain pending until explicit transition, got %q", committed.Execution.Result.Status)
	}

	visibleAuditAgain, err := auditStore.AllContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history after transaction commit: %v", err)
	}
	if len(visibleAuditAgain) != 1 || visibleAuditAgain[0] != event {
		t.Fatalf("audit visibility changed after transaction commit: %#v", visibleAuditAgain)
	}
}

func TestPostgresAuditTransactionCrossReadConvergesAfterBothCommits(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_transaction_convergence_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	auditReaderConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open audit reader connection: %v", err)
	}
	defer auditReaderConn.Close()
	if _, err := auditReaderConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set audit reader search path: %v", err)
	}
	auditStore, err := NewPostgresTransactionAuditStore(auditReaderConn)
	if err != nil {
		t.Fatal(err)
	}

	transactionReaderConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open transaction reader connection: %v", err)
	}
	defer transactionReaderConn.Close()
	if _, err := transactionReaderConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set transaction reader search path: %v", err)
	}
	transactionStore, err := NewPostgresTransactionStore(transactionReaderConn)
	if err != nil {
		t.Fatal(err)
	}

	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "cross-read-convergence-ref",
		Amount:      20000,
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	first := TransactionAuditEvent{
		ReferenceID:  request.ReferenceID,
		Action:       "PURCHASE_PENDING",
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "pending committed before transaction state",
		CreatedAt:    createdAt,
	}
	second := TransactionAuditEvent{
		ReferenceID:  request.ReferenceID,
		Action:       "PURCHASE_RESULT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "terminal evidence committed before transaction state",
		CreatedAt:    createdAt,
	}

	writerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open convergence writer connection: %v", err)
	}
	defer writerConn.Close()
	if _, err := writerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set convergence writer search path: %v", err)
	}

	auditTx, err := writerConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin audit commit transaction: %v", err)
	}
	for _, event := range []TransactionAuditEvent{first, second} {
		if _, err := auditTx.ExecContext(ctx, postgresAuditAppendSQL,
			event.ReferenceID, event.Action, event.Previous, event.Next,
			event.ProviderName, event.Message, event.CreatedAt.UTC(),
		); err != nil {
			auditTx.Rollback()
			t.Fatalf("stage audit event %s: %v", event.Action, err)
		}
	}
	if err := auditTx.Commit(); err != nil {
		t.Fatalf("commit audit evidence: %v", err)
	}

	pending := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				CustomerNo:  request.CustomerNo,
				ProductCode: request.ProductCode,
				Status:       provider.StatusPending,
			},
		},
		Version: 1,
	}
	if err := transactionStore.PutContext(ctx, pending); err != nil {
		t.Fatalf("commit transaction state after audit evidence: %v", err)
	}

	auditEvents, err := auditStore.AllContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read converged audit history: %v", err)
	}
	if len(auditEvents) != 2 || auditEvents[0] != first || auditEvents[1] != second {
		t.Fatalf("unexpected converged audit history: %#v", auditEvents)
	}

	transactionState, ok, err := transactionStore.GetContextE(ctx, request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read converged transaction state: ok=%v err=%v", ok, err)
	}
	if transactionState.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("audit terminal-looking evidence must not upgrade transaction state, got %q", transactionState.Execution.Result.Status)
	}
}

func TestPostgresTransactionAuditStoreCrossDomainReadSurvivesRestart(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_tx_restart_crossread_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	writerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open writer connection: %v", err)
	}
	defer writerConn.Close()
	if _, err := writerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set writer search path: %v", err)
	}
	auditStore, err := NewPostgresTransactionAuditStore(writerConn)
	if err != nil {
		t.Fatal(err)
	}
	transactionStore, err := NewPostgresTransactionStore(writerConn)
	if err != nil {
		t.Fatal(err)
	}

	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "cross-domain-restart-ref",
		Amount:      20000,
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	first := TransactionAuditEvent{
		ReferenceID:  request.ReferenceID,
		Action:       "PURCHASE_PENDING",
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "pending",
		CreatedAt:    createdAt,
	}
	second := TransactionAuditEvent{
		ReferenceID:  request.ReferenceID,
		Action:       "PURCHASE_RESULT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "terminal evidence",
		CreatedAt:    createdAt,
	}
	for _, event := range []TransactionAuditEvent{first, second} {
		if err := auditStore.AppendContext(ctx, event); err != nil {
			t.Fatalf("append audit event %s: %v", event.Action, err)
		}
	}

	state := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				CustomerNo:  request.CustomerNo,
				ProductCode: request.ProductCode,
				Status:      provider.StatusPending,
			},
		},
		Version: 1,
	}
	if err := transactionStore.PutContext(ctx, state); err != nil {
		t.Fatalf("persist pending transaction state: %v", err)
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

	auditReaderConn, err := reopened.Conn(ctx)
	if err != nil {
		t.Fatalf("open fresh audit reader connection: %v", err)
	}
	defer auditReaderConn.Close()
	if _, err := auditReaderConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set fresh audit reader search path: %v", err)
	}
	freshAuditStore, err := NewPostgresTransactionAuditStore(auditReaderConn)
	if err != nil {
		t.Fatal(err)
	}

	transactionReaderConn, err := reopened.Conn(ctx)
	if err != nil {
		t.Fatalf("open fresh transaction reader connection: %v", err)
	}
	defer transactionReaderConn.Close()
	if _, err := transactionReaderConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set fresh transaction reader search path: %v", err)
	}
	freshTransactionStore, err := NewPostgresTransactionStore(transactionReaderConn)
	if err != nil {
		t.Fatal(err)
	}

	auditEvents, err := freshAuditStore.AllContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read audit history after restart: %v", err)
	}
	if len(auditEvents) != 2 || auditEvents[0] != first || auditEvents[1] != second {
		t.Fatalf("audit history changed after restart: %#v", auditEvents)
	}

	transactionState, ok, err := freshTransactionStore.GetContextE(ctx, request.ReferenceID)
	if err != nil || !ok {
		t.Fatalf("read transaction state after restart: ok=%v err=%v", ok, err)
	}
	if transactionState.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("terminal-looking audit evidence must not upgrade transaction after restart, got %q", transactionState.Execution.Result.Status)
	}
	if transactionState.Request != request || transactionState.Execution.ProviderName != "mock" {
		t.Fatalf("transaction state changed after restart: %#v", transactionState)
	}
}


func TestPostgresTransactionAuditStoreCrossDomainReadAfterRestartIsIdempotent(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_tx_restart_idempotency_" + strconv.FormatInt(time.Now().UnixNano(), 10)
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

	writerConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("open writer connection: %v", err)
	}
	defer writerConn.Close()
	if _, err := writerConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("set writer search path: %v", err)
	}

	auditStore, err := NewPostgresTransactionAuditStore(writerConn)
	if err != nil {
		t.Fatal(err)
	}
	transactionStore, err := NewPostgresTransactionStore(writerConn)
	if err != nil {
		t.Fatal(err)
	}

	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "cross-domain-restart-idempotency-ref",
		Amount:      20000,
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	first := TransactionAuditEvent{
		ReferenceID:  request.ReferenceID,
		Action:       "PURCHASE_PENDING",
		Next:         string(provider.StatusPending),
		ProviderName: "mock",
		Message:      "pending",
		CreatedAt:    createdAt,
	}
	second := TransactionAuditEvent{
		ReferenceID:  request.ReferenceID,
		Action:       "PURCHASE_RESULT",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "terminal-looking evidence only",
		CreatedAt:    createdAt,
	}
	for _, event := range []TransactionAuditEvent{first, second} {
		if err := auditStore.AppendContext(ctx, event); err != nil {
			t.Fatalf("append audit event %s: %v", event.Action, err)
		}
	}

	state := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				CustomerNo:  request.CustomerNo,
				ProductCode: request.ProductCode,
				Status:      provider.StatusPending,
			},
		},
		Version: 1,
	}
	if err := transactionStore.PutContext(ctx, state); err != nil {
		t.Fatalf("persist pending transaction state: %v", err)
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

	readOnce := func() ([]TransactionAuditEvent, TransactionState, error) {
		auditConn, err := reopened.Conn(ctx)
		if err != nil {
			return nil, TransactionState{}, err
		}
		defer auditConn.Close()
		if _, err := auditConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
			return nil, TransactionState{}, err
		}
		auditReader, err := NewPostgresTransactionAuditStore(auditConn)
		if err != nil {
			return nil, TransactionState{}, err
		}

		transactionConn, err := reopened.Conn(ctx)
		if err != nil {
			return nil, TransactionState{}, err
		}
		defer transactionConn.Close()
		if _, err := transactionConn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
			return nil, TransactionState{}, err
		}
		transactionReader, err := NewPostgresTransactionStore(transactionConn)
		if err != nil {
			return nil, TransactionState{}, err
		}

		auditEvents, err := auditReader.AllContextE(ctx, request.ReferenceID)
		if err != nil {
			return nil, TransactionState{}, err
		}
		transactionState, ok, err := transactionReader.GetContextE(ctx, request.ReferenceID)
		if err != nil {
			return nil, TransactionState{}, err
		}
		if !ok {
			return nil, TransactionState{}, errors.New("transaction state missing after restart")
		}
		return auditEvents, transactionState, nil
	}

	beforeAudit, beforeTransaction, err := readOnce()
	if err != nil {
		t.Fatalf("first restart read: %v", err)
	}
	afterAudit, afterTransaction, err := readOnce()
	if err != nil {
		t.Fatalf("second restart read: %v", err)
	}

	if len(beforeAudit) != 2 || beforeAudit[0] != first || beforeAudit[1] != second {
		t.Fatalf("unexpected first restart audit read: %#v", beforeAudit)
	}
	if len(afterAudit) != len(beforeAudit) || afterAudit[0] != beforeAudit[0] || afterAudit[1] != beforeAudit[1] {
		t.Fatalf("repeated restart audit read changed history: before=%#v after=%#v", beforeAudit, afterAudit)
	}
	if beforeTransaction != afterTransaction {
		t.Fatalf("repeated restart transaction read changed durable state: before=%#v after=%#v", beforeTransaction, afterTransaction)
	}
	if beforeTransaction.Execution.Result.Status != provider.StatusPending {
		t.Fatalf("terminal-looking audit history must not upgrade transaction state, got %q", beforeTransaction.Execution.Result.Status)
	}

	var auditCount, transactionCount int
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+schema+".provider_transaction_audit WHERE reference_id=$1", request.ReferenceID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows after repeated reads: %v", err)
	}
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+schema+".provider_transactions WHERE reference_id=$1", request.ReferenceID).Scan(&transactionCount); err != nil {
		t.Fatalf("count transaction rows after repeated reads: %v", err)
	}
	if auditCount != 2 || transactionCount != 1 {
		t.Fatalf("repeated restart reads must not mutate persistence: audit=%d transaction=%d", auditCount, transactionCount)
	}
}


func TestPostgresTransactionAuditStoreConcurrentCrossDomainReadsStable(t *testing.T) {
	db := postgresIntegrationDB(t)
	schema := "audit_transaction_cross_read_concurrency_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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

	request := PurchaseRequest{
		ProductCode: "pln20",
		CustomerNo:  "08123456789",
		ReferenceID: "cross-read-concurrency-ref",
		Amount:      20000,
	}
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	auditEvents := []TransactionAuditEvent{
		{
			ReferenceID:  request.ReferenceID,
			Action:       "PURCHASE_PENDING",
			Next:         string(provider.StatusPending),
			ProviderName: "mock",
			Message:      "pending",
			CreatedAt:    createdAt,
		},
		{
			ReferenceID:  request.ReferenceID,
			Action:       "PURCHASE_RESULT",
			Previous:     string(provider.StatusPending),
			Next:         string(provider.StatusSuccess),
			ProviderName: "mock",
			Message:      "success",
			CreatedAt:    createdAt,
		},
	}
	for _, event := range auditEvents {
		if err := auditStore.AppendContext(ctx, event); err != nil {
			t.Fatalf("append audit event %s: %v", event.Action, err)
		}
	}
	transaction := TransactionState{
		Request: request,
		Execution: PurchaseExecution{
			ProviderName: "mock",
			Result: provider.PurchaseResult{
				ReferenceID: request.ReferenceID,
				CustomerNo:  request.CustomerNo,
				ProductCode: request.ProductCode,
				Status:      provider.StatusSuccess,
				ProviderCode: "00",
				Message:      "success",
				Price:        request.Amount,
			},
		},
		Version: 2,
	}
	if err := transactionStore.PutContext(ctx, transaction); err != nil {
		t.Fatalf("persist transaction state: %v", err)
	}

	baselineAudit, err := auditStore.AllContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read baseline audit history: %v", err)
	}
	baselineTransaction, ok, err := transactionStore.GetContextE(ctx, request.ReferenceID)
	if err != nil {
		t.Fatalf("read baseline transaction state: %v", err)
	}
	if !ok {
		t.Fatal("baseline transaction state missing")
	}
	if len(baselineAudit) != len(auditEvents) {
		t.Fatalf("unexpected baseline audit history: %#v", baselineAudit)
	}

	const readers = 16
	const rounds = 20
	start := make(chan struct{})
	errs := make(chan error, readers)
	var wg sync.WaitGroup
	wg.Add(readers)

	for worker := 0; worker < readers; worker++ {
		go func(worker int) {
			defer wg.Done()
			<-start
			for round := 0; round < rounds; round++ {
				conn, err := db.Conn(ctx)
				if err != nil {
					errs <- fmt.Errorf("worker %d round %d open connection: %w", worker, round, err)
					return
				}
				if _, err := conn.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d set search path: %w", worker, round, err)
					return
				}
				readerAudit, err := NewPostgresTransactionAuditStore(conn)
				if err != nil {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d audit store: %w", worker, round, err)
					return
				}
				readerTransaction, err := NewPostgresTransactionStore(conn)
				if err != nil {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d transaction store: %w", worker, round, err)
					return
				}

				gotAudit, err := readerAudit.AllContextE(ctx, request.ReferenceID)
				if err != nil {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d audit read: %w", worker, round, err)
					return
				}
				gotTransaction, ok, err := readerTransaction.GetContextE(ctx, request.ReferenceID)
				if err != nil {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d transaction read: %w", worker, round, err)
					return
				}
				if !ok {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d transaction disappeared", worker, round)
					return
				}
				if len(gotAudit) != len(baselineAudit) {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d audit count changed: got=%d want=%d", worker, round, len(gotAudit), len(baselineAudit))
					return
				}
				for j := range baselineAudit {
					if gotAudit[j] != baselineAudit[j] {
						_ = conn.Close()
						errs <- fmt.Errorf("worker %d round %d audit snapshot changed at %d: got=%#v want=%#v", worker, round, j, gotAudit[j], baselineAudit[j])
						return
					}
				}
				if gotTransaction != baselineTransaction {
					_ = conn.Close()
					errs <- fmt.Errorf("worker %d round %d transaction snapshot changed: got=%#v want=%#v", worker, round, gotTransaction, baselineTransaction)
					return
				}
				if err := conn.Close(); err != nil {
					errs <- fmt.Errorf("worker %d round %d close connection: %w", worker, round, err)
					return
				}
			}
		}(worker)
	}

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
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
