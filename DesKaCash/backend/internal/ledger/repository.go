package ledger

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("ledger record not found")

// Repository abstracts ledger persistence from the business service.
type Repository interface {
	GetAccount(ctx context.Context, id string) (Account, error)
	CreateAccount(ctx context.Context, account Account) error
	SaveAccount(ctx context.Context, account Account) error

	GetTransaction(ctx context.Context, id string) (Transaction, error)
	CreateTransaction(ctx context.Context, tx Transaction) error

	CreateEntry(ctx context.Context, entry Entry) error
	ListEntries(ctx context.Context, accountID string) ([]Entry, error)
}
