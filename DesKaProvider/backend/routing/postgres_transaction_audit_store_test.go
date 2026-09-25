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

func TestPostgresTransactionAuditStoreAppendContextPropagatesCancellation(t *testing.T) {
	store, err := NewPostgresTransactionAuditStore(&fakeDB{})
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
