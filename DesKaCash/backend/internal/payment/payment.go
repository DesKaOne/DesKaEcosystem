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
// DesKaCash keeps only normalized/opaque gateway references; provider routing
// and provider-specific identifiers stay behind the DesKaProvider boundary.
type Payment struct {
	ID               string
	AccountID        string
	ExternalReference string
	IdempotencyKey   string
	Amount           ledger.Money
	Status           Status
	Reference        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewPayment(id, accountID, idempotencyKey string, amount ledger.Money) (Payment, error) {
	if id == "" || accountID == "" || idempotencyKey == "" || amount.BaseUnits <= 0 {
		return Payment{}, ErrInvalidPayment
	}

	now := time.Now().UTC()
	return Payment{
		ID:             id,
		AccountID:      accountID,
		IdempotencyKey: idempotencyKey,
		Amount:         amount,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}
