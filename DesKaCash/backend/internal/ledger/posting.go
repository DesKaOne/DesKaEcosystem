package ledger

import "errors"

type PostingType string

const (
	PostingDebit  PostingType = "debit"
	PostingCredit PostingType = "credit"
)

var ErrInvalidPosting = errors.New("invalid ledger posting")

// Posting is an immutable financial effect attached to a ledger transaction.
// A balanced transaction must contain at least one debit and one credit posting.
type Posting struct {
	ID            string
	TransactionID string
	AccountID     string
	Asset         string
	Type          PostingType
	Amount        Money
	Reference     string
}

func NewPosting(id, transactionID, accountID, asset string, postingType PostingType, amount Money, reference string) (Posting, error) {
	if id == "" || transactionID == "" || accountID == "" || asset == "" || amount.BaseUnits <= 0 {
		return Posting{}, ErrInvalidPosting
	}
	switch postingType {
	case PostingDebit, PostingCredit:
	default:
		return Posting{}, ErrInvalidPosting
	}
	return Posting{
		ID:            id,
		TransactionID: transactionID,
		AccountID:     accountID,
		Asset:         asset,
		Type:          postingType,
		Amount:        amount,
		Reference:     reference,
	}, nil
}
