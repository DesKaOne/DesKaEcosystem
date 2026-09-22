package payment

import (
	"context"
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

var (
	ErrPaymentNotFound        = errors.New("payment not found")
	ErrWebhookAmountMismatch  = errors.New("webhook amount mismatch")
	ErrWebhookPaymentMismatch = errors.New("webhook payment mismatch")
	ErrInvalidPaymentStatus   = errors.New("invalid payment status transition")
)

type PaymentStore interface {
	Get(ctx context.Context, id string) (Payment, error)
	Save(ctx context.Context, payment Payment) error
}

type LedgerCreditor interface {
	ApplyCredit(ctx context.Context, tx ledger.Transaction, reference string) error
}

type Reconciler struct {
	payments PaymentStore
	webhooks WebhookStore
	ledger   LedgerCreditor
}

func NewReconciler(payments PaymentStore, webhooks WebhookStore, ledger LedgerCreditor) *Reconciler {
	return &Reconciler{payments: payments, webhooks: webhooks, ledger: ledger}
}

func validPaymentTransition(current, next Status) bool {
	if current == next {
		return true
	}

	switch current {
	case StatusPending:
		return next == StatusSucceeded || next == StatusFailed || next == StatusExpired
	case StatusSucceeded:
		return next == StatusReversed || next == StatusRefunded
	default:
		return false
	}
}

func (r *Reconciler) ReconcileWebhook(ctx context.Context, event WebhookEvent, paymentID string) (Payment, error) {
	if err := ctx.Err(); err != nil {
		return Payment{}, err
	}

	payment, err := r.payments.Get(ctx, paymentID)
	if err != nil {
		return Payment{}, err
	}

	if event.Provider != payment.Provider || event.ProviderID != payment.ProviderID {
		return Payment{}, ErrWebhookPaymentMismatch
	}
	if event.Amount != payment.Amount.BaseUnits {
		return Payment{}, ErrWebhookAmountMismatch
	}

	nextStatus := event.Status
	switch event.Status {
	case StatusSucceeded, StatusFailed, StatusExpired, StatusReversed, StatusRefunded:
	default:
		nextStatus = StatusPending
	}

	if !validPaymentTransition(payment.Status, nextStatus) {
		return Payment{}, ErrInvalidPaymentStatus
	}

	if nextStatus == StatusSucceeded {
		tx, err := ledger.NewTransaction(payment.ID, payment.AccountID, payment.Amount, "payment")
		if err != nil {
			return Payment{}, err
		}
		if err := r.ledger.ApplyCredit(ctx, tx, payment.Reference); err != nil {
			if !errors.Is(err, ledger.ErrDuplicateTransaction) {
				return Payment{}, err
			}
		}
	}

	payment.Status = nextStatus
	if !event.ReceivedAt.IsZero() {
		payment.UpdatedAt = event.ReceivedAt
	} else if !event.OccurredAt.IsZero() {
		payment.UpdatedAt = event.OccurredAt
	}

	if err := r.payments.Save(ctx, payment); err != nil {
		return Payment{}, err
	}

	if err := r.webhooks.Record(ctx, event); err != nil {
		if errors.Is(err, ErrDuplicateWebhook) {
			return payment, nil
		}
		return Payment{}, err
	}

	return payment, nil
}
