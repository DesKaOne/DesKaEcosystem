package payment

import (
	"context"
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
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

type LedgerReconciler interface {
	ApplyCredit(ctx context.Context, tx ledger.Transaction, reference string) error
	ApplyDebit(ctx context.Context, tx ledger.Transaction, reference string) error
}

type Reconciler struct {
	payments PaymentStore
	webhooks WebhookStore
	ledger   LedgerReconciler
}

func NewReconciler(payments PaymentStore, webhooks WebhookStore, ledger LedgerReconciler) *Reconciler {
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

func validWebhookStatus(status Status) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusExpired, StatusReversed, StatusRefunded:
		return true
	default:
		return false
	}
}

func (r *Reconciler) ReconcileWebhook(ctx context.Context, event WebhookEvent, paymentID string) (Payment, error) {
	if err := ctx.Err(); err != nil {
		return Payment{}, err
	}

	if event.ID == "" || event.Provider == "" || event.ProviderID == "" || event.Amount <= 0 || !validWebhookStatus(event.Status) {
		return Payment{}, ErrInvalidWebhookEvent
	}

	payment, err := r.payments.Get(ctx, paymentID)
	if err != nil {
		return Payment{}, err
	}

	if existing, err := r.webhooks.Get(ctx, event.ID); err == nil {
		if existing.Provider != event.Provider || existing.ProviderID != event.ProviderID ||
			existing.Status != event.Status || existing.Amount != event.Amount {
			return Payment{}, ErrWebhookPaymentMismatch
		}
		return payment, nil
	} else if !errors.Is(err, ErrWebhookNotFound) {
		return Payment{}, err
	}

	if event.Provider != payment.Provider || event.ProviderID != payment.ProviderID {
		return Payment{}, ErrWebhookPaymentMismatch
	}
	if event.Amount != payment.Amount.BaseUnits {
		return Payment{}, ErrWebhookAmountMismatch
	}

	nextStatus := event.Status
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
	} else if nextStatus == StatusReversed || nextStatus == StatusRefunded {
		txID := payment.ID + ":" + string(nextStatus)
		tx, err := ledger.NewTransaction(txID, payment.AccountID, payment.Amount, "payment_"+string(nextStatus))
		if err != nil {
			return Payment{}, err
		}
		if err := r.ledger.ApplyDebit(ctx, tx, payment.Reference); err != nil {
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
