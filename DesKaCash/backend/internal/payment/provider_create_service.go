package payment

import (
	"context"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

type ProviderCreateService struct {
	store    PaymentStore
	provider Provider
}

func NewProviderCreateService(store PaymentStore, provider Provider) *ProviderCreateService {
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

	providerPayment, err := s.provider.CreatePayment(ctx, payment)
	if err != nil {
		return Payment{}, err
	}

	payment.ProviderID = providerPayment.ID
	payment.Reference = providerPayment.Reference
	payment.Status = providerPayment.Status

	if err := s.store.Create(ctx, payment); err != nil {
		return Payment{}, err
	}

	return payment, nil
}
