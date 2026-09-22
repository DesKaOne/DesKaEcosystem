package ledger

import "errors"

var ErrNotFound = errors.New("ledger record not found")

// Repository abstracts ledger persistence from the business service.
type Repository interface {
	GetAccount(id string) (Account, error)
	SaveAccount(account Account) error

	GetTransaction(id string) (Transaction, error)
	CreateTransaction(tx Transaction) error

	CreateEntry(entry Entry) error
	ListEntries(accountID string) ([]Entry, error)
}
