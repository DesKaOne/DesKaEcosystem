package routing

import (
	"testing"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

func TestMemoryTransactionAuditStoreIsAppendOnly(t *testing.T) {
	store := NewMemoryTransactionAuditStore()
	now := time.Now().UTC()
	event := TransactionAuditEvent{
		ReferenceID:  "ref-1",
		Action:       "PURCHASE_TERMINAL",
		Previous:     string(provider.StatusPending),
		Next:         string(provider.StatusSuccess),
		ProviderName: "mock",
		Message:      "success",
		CreatedAt:    now,
	}
	if err := store.Append(event); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(event); err != nil {
		t.Fatal(err)
	}
	events := store.All("ref-1")
	if len(events) != 2 {
		t.Fatalf("expected two immutable audit events, got %d", len(events))
	}
	if events[0] != event || events[1] != event {
		t.Fatalf("audit events changed: %#v", events)
	}
}

func TestMemoryTransactionAuditStoreRejectsIncompleteEvent(t *testing.T) {
	store := NewMemoryTransactionAuditStore()
	if err := store.Append(TransactionAuditEvent{}); err == nil {
		t.Fatal("expected incomplete audit event to be rejected")
	}
}

func TestMemoryTransactionAuditStoreAllContextEPropagatesCancellation(t *testing.T) {
	store := NewMemoryTransactionAuditStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events, err := store.AllContextE(ctx, "ref-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got events=%#v err=%v", events, err)
	}
	if events != nil {
		t.Fatalf("canceled audit read must not expose history, got %#v", events)
	}
}
