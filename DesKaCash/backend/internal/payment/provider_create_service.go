package payment

import (
	"context"
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

var ErrProviderAmountMismatch = errors.New("provider amount mismatch")

type PaymentLifecycleStore interface {
	PaymentCreator
	Get(ctx context.Context, id string) (Payment, error)
	Save(ctx context.Context, payment Payment) error
}

type ProviderCreateService struct {
	store    PaymentLifecycleStore
	provider Provider
}

func NewProviderCreateService(store PaymentLifecycleStore, provider Provider) *ProviderCreateService {
	return &ProviderCreateService{
		store:    store,
		provider: provider,
	}
}

func (s *ProviderCreateService) Create(ctx context.Context, id, accountID, providerName, idempotencyKey string, amount ledger.Money) (Payment, error) {
	if err := ctx.Err(); err != nil {
		return Payment{}, err
	}

	payment, err := NewPayment(id, accountID, providerName, idempotencyKey, amount)
	if err != nil {
		return Payment{}, err
	}

	if err := s.store.Create(ctx, payment); err != nil {
		return Payment{}, err
	}

	providerPayment, err := s.provider.CreatePayment(ctx, payment)
	if err != nil {
		return payment, err
	}
	if providerPayment.ID == "" {
		return payment, ErrInvalidPayment
	}
	if providerPayment.Amount.BaseUnits != payment.Amount.BaseUnits {
		return payment, ErrProviderAmountMismatch
	}

	payment.ProviderID = providerPayment.ID
	payment.Reference = providerPayment.Reference
	payment.Status = providerPayment.Status

	if err := s.store.Save(ctx, payment); err != nil {
		return payment, err
	}

	return payment, nil
}
