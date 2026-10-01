package ledger

import "time"

type EntryType string

const (
	EntryCredit EntryType = "credit"
	EntryDebit  EntryType = "debit"
)

// Entry is an immutable application-ledger record.
// Persistence and database implementation will be added separately.
type Entry struct {
	ID            string
	AccountID     string
	TransactionID string
	Type          EntryType
	Asset         string
	Amount        Money
	Reference     string
	CreatedAt     time.Time
}
