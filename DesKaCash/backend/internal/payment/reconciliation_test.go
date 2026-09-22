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

type memoryLedgerReconciler struct {
	credits []ledger.Transaction
	debits  []ledger.Transaction
}

func (s *memoryLedgerReconciler) ApplyCredit(ctx context.Context, tx ledger.Transaction, reference string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, existing := range s.credits {
		if existing.ID == tx.ID {
			return ledger.ErrDuplicateTransaction
		}
	}
	s.credits = append(s.credits, tx)
	return nil
}

func (s *memoryLedgerReconciler) ApplyDebit(ctx context.Context, tx ledger.Transaction, reference string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, existing := range s.debits {
		if existing.ID == tx.ID {
			return ledger.ErrDuplicateTransaction
		}
	}
	s.debits = append(s.debits, tx)
	return nil
}

func newReconciler(payment Payment) (*Reconciler, *memoryLedgerReconciler) {
	ledgerStore := &memoryLedgerReconciler{}
	return NewReconciler(newMemoryPaymentStore(payment), NewMemoryWebhookStore(), ledgerStore), ledgerStore
}

func TestReconcilerUpdatesPaymentStatusAndCreditsLedger(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	reconciler, creditor := newReconciler(payment)
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
	if len(creditor.credits) != 1 {
		t.Fatalf("expected one ledger credit, got %d", len(creditor.credits))
	}
	if creditor.credits[0].Amount != payment.Amount {
		t.Fatalf("expected credit amount %v, got %v", payment.Amount, creditor.credits[0].Amount)
	}
}

func TestReconcilerRejectsMismatchedWebhook(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	reconciler, _ := newReconciler(payment)
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

	reconciler, _ := newReconciler(payment)
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

	reconciler, creditor := newReconciler(payment)
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
	if len(creditor.credits) != 1 {
		t.Fatalf("expected one ledger credit, got %d", len(creditor.credits))
	}
}

func TestReconcilerRejectsTerminalStatusRegression(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"
	payment.Status = StatusSucceeded

	reconciler, creditor := newReconciler(payment)
	event := WebhookEvent{
		ID: "event-1", Provider: "demo", ProviderID: "provider-1",
		Status: StatusFailed, Amount: payment.Amount.BaseUnits,
	}

	_, err = reconciler.ReconcileWebhook(context.Background(), event, payment.ID)
	if !errors.Is(err, ErrInvalidPaymentStatus) {
		t.Fatalf("expected invalid payment status, got %v", err)
	}
	if len(creditor.credits) != 0 {
		t.Fatalf("expected no ledger credit, got %d", len(creditor.credits))
	}
}

func TestReconcilerDoesNotReprocessDuplicateWebhookPayload(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	reconciler, creditor := newReconciler(payment)
	event := WebhookEvent{
		ID: "event-1", Provider: "demo", ProviderID: "provider-1",
		Status: StatusSucceeded, Amount: payment.Amount.BaseUnits,
	}

	if _, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID); err != nil {
		t.Fatal(err)
	}

	duplicate := event
	duplicate.Status = StatusFailed
	if _, err := reconciler.ReconcileWebhook(context.Background(), duplicate, payment.ID); !errors.Is(err, ErrWebhookPaymentMismatch) {
		t.Fatalf("expected duplicate payload mismatch, got %v", err)
	}
	if len(creditor.credits) != 1 {
		t.Fatalf("expected one ledger credit, got %d", len(creditor.credits))
	}
}

func TestReconcilerRejectsInvalidWebhookStatus(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"

	reconciler, creditor := newReconciler(payment)
	event := WebhookEvent{
		ID: "event-1", Provider: "demo", ProviderID: "provider-1",
		Status: Status("provider_unknown"), Amount: payment.Amount.BaseUnits,
	}

	_, err = reconciler.ReconcileWebhook(context.Background(), event, payment.ID)
	if !errors.Is(err, ErrInvalidWebhookEvent) {
		t.Fatalf("expected invalid webhook event, got %v", err)
	}
	if len(creditor.credits) != 0 {
		t.Fatalf("expected no ledger credit, got %d", len(creditor.credits))
	}
}


func TestReconcilerReversesSucceededPaymentWithLedgerDebit(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"
	payment.Status = StatusSucceeded

	reconciler, ledgerStore := newReconciler(payment)
	event := WebhookEvent{
		ID: "event-reversal", Provider: "demo", ProviderID: "provider-1",
		Status: StatusReversed, Amount: payment.Amount.BaseUnits,
	}

	got, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusReversed {
		t.Fatalf("expected reversed status, got %q", got.Status)
	}
	if len(ledgerStore.debits) != 1 {
		t.Fatalf("expected one ledger debit, got %d", len(ledgerStore.debits))
	}
	if ledgerStore.debits[0].ID != "pay-1:reversed" {
		t.Fatalf("unexpected debit transaction id %q", ledgerStore.debits[0].ID)
	}
}

func TestReconcilerRefundsSucceededPaymentWithLedgerDebit(t *testing.T) {
	payment, err := NewPayment("pay-1", "acct-1", "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil {
		t.Fatal(err)
	}
	payment.ProviderID = "provider-1"
	payment.Status = StatusSucceeded

	reconciler, ledgerStore := newReconciler(payment)
	event := WebhookEvent{
		ID: "event-refund", Provider: "demo", ProviderID: "provider-1",
		Status: StatusRefunded, Amount: payment.Amount.BaseUnits,
	}

	got, err := reconciler.ReconcileWebhook(context.Background(), event, payment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRefunded {
		t.Fatalf("expected refunded status, got %q", got.Status)
	}
	if len(ledgerStore.debits) != 1 {
		t.Fatalf("expected one ledger debit, got %d", len(ledgerStore.debits))
	}
	if ledgerStore.debits[0].ID != "pay-1:refunded" {
		t.Fatalf("unexpected debit transaction id %q", ledgerStore.debits[0].ID)
	}
}

func TestReconcilerReversalDebitsActualLedgerBalance(t *testing.T) {
	ctx := context.Background()
	account, err := ledger.NewAccount("acct-1", "user-1")
	if err != nil { t.Fatal(err) }
	store := ledger.NewMemoryRepository()
	if err := store.CreateAccount(ctx, account); err != nil { t.Fatal(err) }

	seed, err := ledger.NewTransaction("seed-1", account.ID, ledger.FromDIDR(100), "seed")
	if err != nil { t.Fatal(err) }
	if err := store.ApplyCredit(ctx, seed, "seed"); err != nil { t.Fatal(err) }

	payment, err := NewPayment("pay-1", account.ID, "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil { t.Fatal(err) }
	payment.ProviderID = "provider-1"
	payment.Status = StatusSucceeded
	payments := newMemoryPaymentStore(payment)
	reconciler := NewReconciler(payments, NewMemoryWebhookStore(), store)

	event := WebhookEvent{ID: "event-reversal", Provider: "demo", ProviderID: "provider-1", Status: StatusReversed, Amount: payment.Amount.BaseUnits}
	got, err := reconciler.ReconcileWebhook(ctx, event, payment.ID)
	if err != nil { t.Fatal(err) }
	if got.Status != StatusReversed { t.Fatalf("expected reversed status, got %q", got.Status) }

	updated, err := store.GetAccount(ctx, account.ID)
	if err != nil { t.Fatal(err) }
	if updated.Balance.BaseUnits != 0 { t.Fatalf("expected zero balance after reversal, got %d", updated.Balance.BaseUnits) }
	entries, err := store.ListEntries(ctx, account.ID)
	if err != nil { t.Fatal(err) }
	if len(entries) != 2 || entries[1].Type != ledger.EntryDebit { t.Fatalf("expected credit and debit entries, got %#v", entries) }
}

func TestReconcilerRefundInsufficientFundsLeavesPaymentSucceeded(t *testing.T) {
	ctx := context.Background()
	account, err := ledger.NewAccount("acct-1", "user-1")
	if err != nil { t.Fatal(err) }
	store := ledger.NewMemoryRepository()
	if err := store.CreateAccount(ctx, account); err != nil { t.Fatal(err) }

	payment, err := NewPayment("pay-1", account.ID, "demo", "idem-1", ledger.FromDIDR(100))
	if err != nil { t.Fatal(err) }
	payment.ProviderID = "provider-1"
	payment.Status = StatusSucceeded
	payments := newMemoryPaymentStore(payment)
	reconciler := NewReconciler(payments, NewMemoryWebhookStore(), store)

	event := WebhookEvent{ID: "event-refund", Provider: "demo", ProviderID: "provider-1", Status: StatusRefunded, Amount: payment.Amount.BaseUnits}
	if _, err := reconciler.ReconcileWebhook(ctx, event, payment.ID); !errors.Is(err, ledger.ErrInsufficientFunds) { t.Fatalf("expected insufficient funds, got %v", err) }

	stored, err := payments.Get(ctx, payment.ID)
	if err != nil { t.Fatal(err) }
	if stored.Status != StatusSucceeded { t.Fatalf("expected payment to remain succeeded, got %q", stored.Status) }
	entries, err := store.ListEntries(ctx, account.ID)
	if err != nil { t.Fatal(err) }
	if len(entries) != 0 { t.Fatalf("expected no ledger entries after failed refund, got %d", len(entries)) }
}
