package ledger

import (
	"errors"
	"time"
)

var ErrInvalidTransaction = errors.New("invalid ledger transaction")

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusSucceeded TransactionStatus = "succeeded"
	StatusFailed    TransactionStatus = "failed"
	StatusReversed  TransactionStatus = "reversed"
)

// Transaction represents a business transaction that can produce ledger entries.
// It is separate from provider transaction IDs and IndoChain transaction IDs.
type Transaction struct {
	ID          string
	AccountID   string
	Asset       string
	Amount      Money
	Type        string
	Status      TransactionStatus
	ProviderID  string
	Reference   string
	CreatedAt   time.Time
}

func NewTransaction(id, accountID string, amount Money, txType string) (Transaction, error) {
	if id == "" || accountID == "" || amount.BaseUnits <= 0 || txType == "" {
		return Transaction{}, ErrInvalidTransaction
	}

	return Transaction{
		ID:        id,
		AccountID: accountID,
		Asset:     AssetDIDR,
		Amount:    amount,
		Type:      txType,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}, nil
}
