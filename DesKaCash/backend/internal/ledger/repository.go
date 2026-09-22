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

	ApplyCredit(ctx context.Context, tx Transaction, reference string) error
	ApplyDebit(ctx context.Context, tx Transaction, reference string) error
}

// PostingStore persists immutable financial postings.
// Implementations intentionally expose create/read operations only.
type PostingStore interface {
	CreatePosting(ctx context.Context, posting Posting) error
	ListPostings(ctx context.Context, transactionID string) ([]Posting, error)
}
