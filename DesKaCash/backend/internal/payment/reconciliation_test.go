package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

type memoryPaymentStore struct {
	payments map[string]Payment
}

func newMemoryPaymentStore(payment Payment) *memoryPaymentStore {
	return &memoryPaymentStore{payments: map[string]Payment{payment.ID: payment}}
}

func (s *memoryPaymentStore) Get(ctx context.Context, id string) (Payment, error) {
	if err := ctx.Err(); err != nil {
		return Payment{}, err
	}
	payment, ok := s.payments[id]
	if !ok {
		return Payment{}, ErrPaymentNotFound
	}
	return payment, nil
}

func (s *memoryPaymentStore) Save(ctx context.Context, payment Payment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.payments[payment.ID] = payment
	return nil
}

func TestReconcilerUpdatesPaymentStatus(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	store := newMemoryPaymentStore(payment)
	reconciler := NewReconciler(store, NewMemoryWebhookStore())
	receivedAt := time.Now().UTC()
	event := WebhookEvent{
		ID: "event-1", Provider: "demo", ProviderID: "provider-1",
		Status: StatusSucceeded, Amount: payment.Amount.BaseUnits,
		ReceivedAt: receivedAt,
	}

	got, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", got.Status)
	}
	if !got.UpdatedAt.Equal(receivedAt) {
		t.Fatalf("expected updated at %v, got %v", receivedAt, got.UpdatedAt)
	}
}

func TestReconcilerRejectsMismatchedWebhook(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	store := newMemoryPaymentStore(payment)
	reconciler := NewReconciler(store, NewMemoryWebhookStore())
	event := WebhookEvent{
		ID: "event-1", Provider: "other", ProviderID: "provider-1",
		Status: StatusSucceeded, Amount: payment.Amount.BaseUnits,
	}

	if _, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID); !errors.Is(err, ErrWebhookPaymentMismatch) {
		t.Fatalf("expected webhook payment mismatch, got %v", err)
	}
}

func TestReconcilerRejectsMismatchedAmount(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	store := newMemoryPaymentStore(payment)
	reconciler := NewReconciler(store, NewMemoryWebhookStore())
	event := WebhookEvent{
		ID: "event-1", Provider: "demo", ProviderID: "provider-1",
		Status: StatusSucceeded, Amount: payment.Amount.BaseUnits + 1,
	}

	if _, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID); !errors.Is(err, ErrWebhookAmountMismatch) {
		t.Fatalf("expected webhook amount mismatch, got %v", err)
	}
}

func TestReconcilerTreatsDuplicateWebhookAsIdempotent(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	store := newMemoryPaymentStore(payment)
	reconciler := NewReconciler(store, NewMemoryWebhookStore())
	event := WebhookEvent{
		ID: "event-1", Provider: "demo", ProviderID: "provider-1",
		Status: StatusSucceeded, Amount: payment.Amount.BaseUnits,
	}

	if _, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID); err != nil {
		t.Fatal(err)
	}
}
