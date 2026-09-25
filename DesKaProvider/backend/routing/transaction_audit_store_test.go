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
		Previous:     provider.StatusPending,
		Next:         provider.StatusSuccess,
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
