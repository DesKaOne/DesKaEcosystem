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

	if event.Status == StatusSucceeded {
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

	switch event.Status {
	case StatusSucceeded, StatusFailed, StatusExpired, StatusReversed, StatusRefunded:
		payment.Status = event.Status
	default:
		payment.Status = StatusPending
	}

	payment.UpdatedAt = event.ReceivedAt
	if payment.UpdatedAt.IsZero() {
		payment.UpdatedAt = event.OccurredAt
	}
	if payment.UpdatedAt.IsZero() {
		payment.UpdatedAt = event.ReceivedAt
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
