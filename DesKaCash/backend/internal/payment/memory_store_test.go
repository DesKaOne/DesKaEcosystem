package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

func TestMemoryPaymentStoreRejectsDuplicateIdempotencyKey(t *testing.T) {
	store := NewMemoryPaymentStore()

	first, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPayment("pay-2", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Create(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(context.Background(), second); !errors.Is(err, ErrDuplicatePayment) {
		t.Fatalf("expected duplicate payment, got %v", err)
	}
}

func TestMemoryPaymentStoreRoundTripAndSave(t *testing.T) {
	store := NewMemoryPaymentStore()
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Create(context.Background(), payment); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(context.Background(), payment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != payment.Amount {
		t.Fatalf("expected amount %v, got %v", payment.Amount, got.Amount)
	}

	got.Status = StatusSucceeded
	if err := store.Save(context.Background(), got); err != nil {
		t.Fatal(err)
	}

	updated, err := store.Get(context.Background(), payment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", updated.Status)
	}
}

func TestMemoryPaymentStoreMissingRecord(t *testing.T) {
	store := NewMemoryPaymentStore()

	if _, err := store.Get(context.Background(), "missing"); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("expected payment not found, got %v", err)
	}

	if err := store.Save(context.Background(), Payment{ID: "missing", IdempotencyKey: "idem"}); !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("expected payment not found on save, got %v", err)
	}
}
