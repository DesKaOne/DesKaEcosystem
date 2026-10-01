package payment

import (
	"context"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

// DesKaProviderClient is the only payment infrastructure boundary known by
// DesKaCash. It accepts normalized domain requests and returns normalized
// results; provider routing and provider-specific protocols are out of scope.
type DesKaProviderClient interface {
	CreatePayment(ctx context.Context, payment Payment) (ProviderPayment, error)
	GetPayment(ctx context.Context, externalReference string) (ProviderPayment, error)
}

type ProviderPayment struct {
	ExternalReference string
	Status            Status
	Amount            ledger.Money
	Reference         string
}
