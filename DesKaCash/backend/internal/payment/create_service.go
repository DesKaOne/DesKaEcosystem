package payment

import (
	"context"
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/ledger"
)

var ErrPaymentCreationFailed = errors.New("payment creation failed")

type PaymentCreator interface {
	Create(ctx context.Context, payment Payment) error
}

type CreateService struct {
	store PaymentCreator
}

func NewCreateService(store PaymentCreator) *CreateService {
	return &CreateService{store: store}
}

func (s *CreateService) Create(ctx context.Context, id, accountID, provider, idempotencyKey string, amount ledger.Money) (Payment, error) {
	if err := ctx.Err(); err != nil {
		return Payment{}, err
	}

	payment, err := NewPayment(id, accountID, provider, idempotencyKey, amount)
	if err != nil {
		return Payment{}, err
	}

	if err := s.store.Create(ctx, payment); err != nil {
		return Payment{}, err
	}

	return payment, nil
}
