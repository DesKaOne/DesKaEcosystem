package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

func TestCreateServiceCreatesPayment(t *testing.T) {
	store := NewMemoryPaymentStore()
	service := NewCreateService(store)

	payment, err := service.Create(context.Background(), "pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	if payment.Status != StatusPending {
		t.Fatalf("expected pending status, got %q", payment.Status)
	}

	got, err := store.Get(context.Background(), "pay-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.IdempotencyKey != "idem-1" {
		t.Fatalf("expected idempotency key idem-1, got %q", got.IdempotencyKey)
	}
}

func TestCreateServiceRejectsDuplicateIdempotencyKey(t *testing.T) {
	store := NewMemoryPaymentStore()
	service := NewCreateService(store)

	if _, err := service.Create(context.Background(), "pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100)); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Create(context.Background(), "pay-2", "acct-1", "demo", "idem-1", ledger.FromDIDR(100)); !errors.Is(err, ErrDuplicatePayment) {
		t.Fatalf("expected duplicate payment, got %v", err)
	}
}
