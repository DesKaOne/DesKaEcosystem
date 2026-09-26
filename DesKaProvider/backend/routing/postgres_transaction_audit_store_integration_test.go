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
