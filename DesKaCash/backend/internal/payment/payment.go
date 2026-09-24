package payment

import (
	"errors"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/ledger"
)

var ErrInvalidPayment = errors.New("invalid payment")

type Status string

const (
	StatusPending   Status = "pending"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
	StatusReversed  Status = "reversed"
	StatusRefunded  Status = "refunded"
)

// Payment is the application-layer record for an external payment attempt.
// Provider IDs and idempotency keys remain integration-layer identifiers.
type Payment struct {
	ID             string
	AccountID      string
	Provider       string
	ProviderID     string
	IdempotencyKey string
	Amount         ledger.Money
	Status         Status
	Reference      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewPayment(id, accountID, provider, idempotencyKey string, amount ledger.Money) (Payment, error) {
	if id == "" || accountID == "" || provider == "" || idempotencyKey == "" || amount.BaseUnits <= 0 {
		return Payment{}, ErrInvalidPayment
	}

	now := time.Now().UTC()
	return Payment{
		ID:             id,
		AccountID:      accountID,
		Provider:       provider,
		IdempotencyKey: idempotencyKey,
		Amount:         amount,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}
