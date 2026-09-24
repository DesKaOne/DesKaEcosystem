package payment

import (
	"context"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

// Provider abstracts an external payment rail from the application domain.
// Concrete adapters such as Flip or Midtrans implement this boundary.
type Provider interface {
	Name() string
	CreatePayment(ctx context.Context, payment Payment) (ProviderPayment, error)
	GetPayment(ctx context.Context, providerID string) (ProviderPayment, error)
}

type ProviderPayment struct {
	ID        string
	Status    Status
	Amount    ledger.Money
	Reference string
}
