package routing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPostgresTransactionAuditStoreValidation(t *testing.T) {
	store, err := NewPostgresTransactionAuditStore(&postgresStoreDBStub{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(TransactionAuditEvent{Action: "TEST", CreatedAt: time.Now().UTC()}); err == nil {
		t.Fatal("expected missing reference ID to be rejected")
	}
	if err := store.Append(TransactionAuditEvent{ReferenceID: "ref", Action: "TEST"}); err == nil {
		t.Fatal("expected missing created_at to be rejected")
	}
	if err := store.AppendContext(context.Background(), TransactionAuditEvent{ReferenceID: "ref", Action: "TEST"}); err == nil {
		t.Fatal("expected invalid event to be rejected before database access")
	}
}

func TestPostgresTransactionAuditStoreAppendContextPropagatesDatabaseError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	stub := &postgresStoreDBStub{err: wantErr}
	store, err := NewPostgresTransactionAuditStore(stub)
	if err != nil {
		t.Fatal(err)
	}

	err = store.AppendContext(context.Background(), TransactionAuditEvent{
		ReferenceID: "ref",
		Action: "PURCHASE_RESULT",
		CreatedAt: time.Now().UTC(),
	})
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected database append error to propagate, got %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("database append error must not be reported as cancellation: %v", err)
	}
	if stub.query != postgresAuditAppendSQL {
		t.Fatalf("expected audit append SQL, got %q", stub.query)
	}
}

func TestPostgresTransactionAuditStoreAppendContextPropagatesCancellation(t *testing.T) {
	store, err := NewPostgresTransactionAuditStore(&postgresStoreDBStub{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = store.AppendContext(ctx, TransactionAuditEvent{
		ReferenceID: "ref",
		Action: "TEST",
		CreatedAt: time.Now().UTC(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}